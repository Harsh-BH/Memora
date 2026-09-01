package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/memora/cma/configs"
	"github.com/memora/cma/internal/models"
)

// ErrClaudeCLIUnsupported is returned by the ClaudeCLIProvider methods that
// `claude -p` structurally cannot serve: it is a text-in/text-out subprocess,
// so there is no embedding surface and no token-logprob surface (the Messages
// API has no logprobs at all, so ScoreDIG and GetTokenProbabilities are a
// permanent capability gap, not a missing credential). Embed/EmbedBatch are
// served by LocalEmbedProvider and must NOT be routed here.
var ErrClaudeCLIUnsupported = errors.New("llm: not available on ClaudeCLIProvider (`claude -p` is text-only: no embeddings, no logprobs)")

// claudeCLIFlags are the fixed arguments for every invocation.
//
//   - --strict-mcp-config + an empty --mcp-config stops the CLI loading the
//     host's MCP servers (latency, stderr noise, and tool surface we do not want).
//   - --no-session-persistence stops one session file being written per cluster.
//   - --disallowed-tools is a trust boundary, not an optimisation: episode
//     content is user data being pasted into a prompt run by a tool-capable
//     agent, so the agent gets no filesystem, shell or network tools.
var claudeCLIFlags = []string{
	"-p",
	"--strict-mcp-config", "--mcp-config", `{"mcpServers":{}}`,
	"--no-session-persistence",
	"--disallowed-tools", "Bash Read Edit Write NotebookEdit WebFetch WebSearch Glob Grep Task",
}

// ClaudeCLIProvider implements the LLM half of Provider by shelling out to the
// user's logged-in `claude -p` CLI, one process per call. It exists because
// consolidation's Synthesize -> ExtractTriples -> conflict.FindConflicts path
// needs a real LLM and this box has no ANTHROPIC_API_KEY or OPENAI_API_KEY.
//
// It is a drop-in Provider: worker.go is unmodified.
type ClaudeCLIProvider struct {
	bin     string
	args    []string
	timeout time.Duration
	sem     chan struct{} // bounded concurrency: each call forks a node process
}

// NewClaudeCLIProvider resolves the `claude` binary on PATH (override with
// CLAUDE_CLI_BIN) and fails at construction if it is absent, so a missing CLI
// is a startup error rather than a consolidation cycle that logs and continues.
func NewClaudeCLIProvider(cfg configs.LLMConfig) (*ClaudeCLIProvider, error) {
	bin := os.Getenv("CLAUDE_CLI_BIN")
	if bin == "" {
		var err error
		if bin, err = exec.LookPath("claude"); err != nil {
			return nil, fmt.Errorf("claude cli: %w (set CLAUDE_CLI_BIN or install the CLI)", err)
		}
	}

	args := append([]string{}, claudeCLIFlags...)
	if cfg.Model != "" {
		args = append(args, "--model", cfg.Model)
	}

	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	conc := cfg.MaxConcurrency
	if conc <= 0 {
		conc = 2
	}

	return &ClaudeCLIProvider{
		bin:     bin,
		args:    args,
		timeout: timeout,
		sem:     make(chan struct{}, conc),
	}, nil
}

// run sends prompt on stdin and returns trimmed stdout. Empty output is an
// error: a silent empty return is how this project got a consolidation cycle
// that reported success while doing nothing.
func (c *ClaudeCLIProvider) run(ctx context.Context, prompt string) (string, error) {
	select {
	case c.sem <- struct{}{}:
	case <-ctx.Done():
		return "", fmt.Errorf("claude cli: waiting for a slot: %w", ctx.Err())
	}
	defer func() { <-c.sem }()

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.bin, c.args...)
	// Run from a neutral directory: the CLI auto-discovers CLAUDE.md and skills
	// from its cwd, and the server's repo must not leak into a synthesis prompt.
	cmd.Dir = os.TempDir()
	cmd.Stdin = strings.NewReader(prompt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("claude cli: %w after %s (stderr: %.400q)", ctx.Err(), c.timeout, stderr.String())
		}
		return "", fmt.Errorf("claude cli: %w (stderr: %.400q)", err, stderr.String())
	}

	out := strings.TrimSpace(stdout.String())
	if out == "" {
		return "", fmt.Errorf("claude cli: empty output (stderr: %.400q)", stderr.String())
	}
	return out, nil
}

// Synthesize generates a gist proposition from a cluster of episodes.
func (c *ClaudeCLIProvider) Synthesize(ctx context.Context, episodes []models.Episode) (string, error) {
	if len(episodes) == 0 {
		return "", errors.New("claude cli synthesize: no episodes")
	}
	out, err := c.run(ctx, synthesizePrompt(episodes))
	if err != nil {
		return "", fmt.Errorf("claude cli synthesize: %w", err)
	}
	return out, nil
}

// ExtractTriples extracts atomic (Subject, Predicate, Object) triples from text.
func (c *ClaudeCLIProvider) ExtractTriples(ctx context.Context, content string) ([]models.Triple, error) {
	out, err := c.run(ctx, extractTriplesPrompt(content))
	if err != nil {
		return nil, fmt.Errorf("claude cli extract triples: %w", err)
	}
	triples, err := parseTriples(out)
	if err != nil {
		return nil, fmt.Errorf("claude cli extract triples: %w", err)
	}
	return triples, nil
}

// Generate produces a completion for general-purpose use. This is the raw
// primitive the two methods above are built on.
func (c *ClaudeCLIProvider) Generate(ctx context.Context, prompt string) (string, error) {
	return c.run(ctx, prompt)
}

// CountTokens uses the same ~4-chars-per-token heuristic as OpenAIProvider.
func (c *ClaudeCLIProvider) CountTokens(text string) int { return approxTokens(text) }

func (c *ClaudeCLIProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	return nil, ErrClaudeCLIUnsupported
}

func (c *ClaudeCLIProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	return nil, ErrClaudeCLIUnsupported
}

func (c *ClaudeCLIProvider) GetTokenProbabilities(ctx context.Context, text string) ([]TokenProb, error) {
	return nil, ErrClaudeCLIUnsupported
}

func (c *ClaudeCLIProvider) ScoreDIG(ctx context.Context, query string, document string) (float64, error) {
	return 0, ErrClaudeCLIUnsupported
}

var _ Provider = (*ClaudeCLIProvider)(nil)

// --- Parsing ---

// parseTriples pulls a JSON array of triples out of raw CLI output that may be
// wrapped in a markdown fence or in prose. Scanning to the first '[' skips any
// leading fence or preamble, and json.Decoder stops at the end of the first
// complete value, so trailing prose and a closing fence need no handling.
//
// Every failure path returns an error carrying the raw output. Nothing here
// may return an empty slice with a nil error unless the model genuinely said
// there are no facts.
func parseTriples(raw string) ([]models.Triple, error) {
	i := strings.Index(raw, "[")
	if i < 0 {
		return nil, fmt.Errorf("no JSON array in output: %.400q", raw)
	}

	var decoded []models.Triple
	if err := json.NewDecoder(strings.NewReader(raw[i:])).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("parse: %w (output: %.400q)", err, raw)
	}

	kept := make([]models.Triple, 0, len(decoded))
	for _, t := range decoded {
		if t.Subject == "" || t.Predicate == "" || t.Object == "" {
			continue
		}
		kept = append(kept, t)
	}
	if len(decoded) > 0 && len(kept) == 0 {
		return nil, fmt.Errorf("all %d triples had an empty subject/predicate/object: %.400q", len(decoded), raw)
	}
	return kept, nil
}

// --- Prompts, shared with OpenAIProvider so both providers ask for the same thing ---

func synthesizePrompt(episodes []models.Episode) string {
	var sb strings.Builder
	for i, ep := range episodes {
		fmt.Fprintf(&sb, "Episode %d (t=%s): %s\n", i+1, ep.Timestamp.Format("2006-01-02T15:04"), ep.Content)
	}
	return fmt.Sprintf(SynthesizePromptTemplate, sb.String())
}

// SynthesizePromptTemplate and ExtractTriplesPromptTemplate are exported so a
// run can freeze the exact prompt text into its artifact without a copy that
// can drift from the code that actually ran.
const SynthesizePromptTemplate = `Synthesize the following episodic memory fragments into a single concise semantic proposition.
The proposition should capture the core factual knowledge that persists across episodes.
Be atomic and precise. Return only the proposition text.

Fragments:
%s`

func extractTriplesPrompt(content string) string {
	return fmt.Sprintf(ExtractTriplesPromptTemplate, content)
}

const ExtractTriplesPromptTemplate = `Extract all factual relationships from the following text as atomic triples.
Return a JSON array where each element has:
- "subject": the entity performing or being described
- "predicate": the relationship or action
- "object": the target entity or value
- "confidence": a float between 0.0 and 1.0 indicating certainty

Only extract clearly stated facts. Do not infer or hallucinate relationships.
Return ONLY valid JSON, no markdown formatting.

Text:
%s`

// approxTokens is the ~4-chars-per-token heuristic used by every provider here
// that has no real tokenizer (LocalEmbedProvider has one; these do not).
func approxTokens(text string) int {
	count := len(text) / 4
	if count == 0 && len(text) > 0 {
		count = 1
	}
	return count
}
