package llm

import (
	"context"
	"errors"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/memora/cma/configs"
	"github.com/memora/cma/internal/models"
)

// TestParseTriples runs over recorded `claude -p` output shapes. No network,
// no subprocess. The malformed cases must ERROR, never return an empty slice
// with a nil error -- a silent empty return is how consolidation ended up
// reporting success while doing nothing.
func TestParseTriples(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    []models.Triple
		wantErr bool
	}{
		{
			name: "bare json array (observed: claude -p returns this unfenced)",
			raw:  `[{"subject":"I","predicate":"works at","object":"Google","confidence":0.95}]`,
			want: []models.Triple{{Subject: "I", Predicate: "works at", Object: "Google", Confidence: 0.95}},
		},
		{
			name: "markdown fenced",
			raw: "```json\n" +
				`[{"subject":"Harsh","predicate":"lives in","object":"Bangalore","confidence":0.9}]` +
				"\n```",
			want: []models.Triple{{Subject: "Harsh", Predicate: "lives in", Object: "Bangalore", Confidence: 0.9}},
		},
		{
			name: "bare fence with no language tag",
			raw:  "```\n[{\"subject\":\"a\",\"predicate\":\"b\",\"object\":\"c\",\"confidence\":0.5}]\n```",
			want: []models.Triple{{Subject: "a", Predicate: "b", Object: "c", Confidence: 0.5}},
		},
		{
			name: "prose before and after",
			raw: "Here are the extracted triples:\n\n" +
				`[{"subject":"user","predicate":"prefers","object":"dark mode","confidence":0.8}]` +
				"\n\nLet me know if you'd like me to extract more.",
			want: []models.Triple{{Subject: "user", Predicate: "prefers", Object: "dark mode", Confidence: 0.8}},
		},
		{
			name: "prose plus fence plus prose",
			raw:  "I found 2 facts.\n```json\n[{\"subject\":\"x\",\"predicate\":\"is\",\"object\":\"y\",\"confidence\":1},{\"subject\":\"p\",\"predicate\":\"is\",\"object\":\"q\",\"confidence\":0.4}]\n```\nDone.",
			want: []models.Triple{
				{Subject: "x", Predicate: "is", Object: "y", Confidence: 1},
				{Subject: "p", Predicate: "is", Object: "q", Confidence: 0.4},
			},
		},
		{
			name: "genuinely no facts is not an error",
			raw:  "[]",
			want: []models.Triple{},
		},
		{
			name: "missing confidence parses as zero, not an error",
			raw:  `[{"subject":"I","predicate":"like","object":"tea"}]`,
			want: []models.Triple{{Subject: "I", Predicate: "like", Object: "tea", Confidence: 0}},
		},
		{
			name: "entries with empty fields are dropped, valid ones kept",
			raw:  `[{"subject":"","predicate":"is","object":"y"},{"subject":"a","predicate":"b","object":"c","confidence":0.3}]`,
			want: []models.Triple{{Subject: "a", Predicate: "b", Object: "c", Confidence: 0.3}},
		},
		{
			name:    "every entry empty is an error, not an empty result",
			raw:     `[{"subject":"","predicate":"","object":""},{"subject":"a","predicate":"","object":"c"}]`,
			wantErr: true,
		},
		{
			name:    "truncated array",
			raw:     `[{"subject":"I","predicate":"works at","object":"Goog`,
			wantErr: true,
		},
		{
			name:    "refusal prose with no json at all",
			raw:     "I'm not able to extract triples from that text.",
			wantErr: true,
		},
		{
			name:    "empty output",
			raw:     "",
			wantErr: true,
		},
		{
			name:    "wrong json shape (object, not array of triples)",
			raw:     `[{"triples": "none"}]`,
			wantErr: true, // decodes, but every field is empty -> loud failure
		},
		{
			name:    "array of scalars",
			raw:     `["works at Google"]`,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseTriples(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got %d triples: %+v", len(got), got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %d triples %+v, want %d %+v", len(got), got, len(tc.want), tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("triple %d: got %+v, want %+v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestClaudeCLIRunSubprocess exercises the exec path with coreutils instead of
// the real CLI: no network, no API cost, but the real stdin wiring, exit-code
// handling, empty-output guard and timeout.
func TestClaudeCLIRunSubprocess(t *testing.T) {
	newProv := func(bin string, args []string, timeout time.Duration) *ClaudeCLIProvider {
		return &ClaudeCLIProvider{bin: bin, args: args, timeout: timeout, sem: make(chan struct{}, 1)}
	}

	t.Run("prompt reaches stdin and stdout comes back trimmed", func(t *testing.T) {
		out, err := newProv("/bin/cat", nil, 5*time.Second).run(context.Background(), "hello\n")
		if err != nil {
			t.Fatalf("run: %v", err)
		}
		if out != "hello" {
			t.Fatalf("got %q, want %q", out, "hello")
		}
	})

	t.Run("empty output is a loud error", func(t *testing.T) {
		_, err := newProv("/bin/cat", nil, 5*time.Second).run(context.Background(), "   \n\t ")
		if err == nil || !strings.Contains(err.Error(), "empty output") {
			t.Fatalf("want empty-output error, got %v", err)
		}
	})

	t.Run("non-zero exit is an error", func(t *testing.T) {
		_, err := newProv("/bin/false", nil, 5*time.Second).run(context.Background(), "x")
		if err == nil {
			t.Fatal("want error from a failing binary")
		}
	})

	t.Run("timeout fires", func(t *testing.T) {
		_, err := newProv("/bin/sleep", []string{"10"}, 50*time.Millisecond).run(context.Background(), "x")
		if err == nil || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("want DeadlineExceeded, got %v", err)
		}
	})

	t.Run("caller cancellation while the concurrency slot is busy", func(t *testing.T) {
		p := newProv("/bin/cat", nil, 5*time.Second)
		p.sem <- struct{}{} // occupy the only slot
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := p.run(ctx, "x")
		if err == nil || !strings.Contains(err.Error(), "waiting for a slot") {
			t.Fatalf("want slot-wait error, got %v", err)
		}
	})
}

// TestClaudeCLIUnsupported pins the methods that must NOT be routed here:
// Embed/EmbedBatch are served by LocalEmbedProvider, and logprobs do not exist.
func TestClaudeCLIUnsupported(t *testing.T) {
	p := &ClaudeCLIProvider{}
	ctx := context.Background()

	if _, err := p.Embed(ctx, "x"); !errors.Is(err, ErrClaudeCLIUnsupported) {
		t.Errorf("Embed: got %v", err)
	}
	if _, err := p.EmbedBatch(ctx, []string{"x"}); !errors.Is(err, ErrClaudeCLIUnsupported) {
		t.Errorf("EmbedBatch: got %v", err)
	}
	if _, err := p.GetTokenProbabilities(ctx, "x"); !errors.Is(err, ErrClaudeCLIUnsupported) {
		t.Errorf("GetTokenProbabilities: got %v", err)
	}
	if _, err := p.ScoreDIG(ctx, "q", "d"); !errors.Is(err, ErrClaudeCLIUnsupported) {
		t.Errorf("ScoreDIG: got %v", err)
	}
	if _, err := p.Synthesize(ctx, nil); err == nil {
		t.Error("Synthesize with no episodes should error")
	}
}

// TestPromptBuilders guards the prompt text now shared with OpenAIProvider.
func TestPromptBuilders(t *testing.T) {
	ts := time.Date(2026, 8, 31, 14, 30, 0, 0, time.UTC)
	got := synthesizePrompt([]models.Episode{{Content: "I moved to Meta", Timestamp: ts}})
	for _, want := range []string{"I moved to Meta", "2026-08-31T14:30", "Episode 1"} {
		if !strings.Contains(got, want) {
			t.Errorf("synthesizePrompt missing %q:\n%s", want, got)
		}
	}
	if p := extractTriplesPrompt("some gist"); !strings.Contains(p, "some gist") || !strings.Contains(p, `"confidence"`) {
		t.Errorf("extractTriplesPrompt malformed:\n%s", p)
	}
	if approxTokens("") != 0 || approxTokens("ab") != 1 || approxTokens("12345678") != 2 {
		t.Error("approxTokens heuristic changed")
	}
}

// TestClaudeCLILive is the only test that spawns the real CLI. It is skipped
// unless CMA_LIVE_CLI=1, so `go test ./...` stays offline and free:
//
//	CMA_LIVE_CLI=1 go test ./internal/llm/ -run TestClaudeCLILive -v -timeout 20m
//
// The fixture is deliberately the supersession case the forgetting experiment
// failed on at the embedding level (employer Stripe -> Google), so the output
// shows whether the fact-level path produces the conflicting triple pair that
// conflict.FindConflicts needs.
func TestClaudeCLILive(t *testing.T) {
	if os.Getenv("CMA_LIVE_CLI") != "1" {
		t.Skip("set CMA_LIVE_CLI=1 to run against the real `claude -p`")
	}

	p, err := NewClaudeCLIProvider(configs.LLMConfig{})
	if err != nil {
		t.Fatalf("NewClaudeCLIProvider: %v", err)
	}

	base := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	episodes := []models.Episode{
		{Content: "I just accepted an offer at Stripe. Starting as a backend engineer next month.", Timestamp: base},
		{Content: "Six months in at Stripe and the payments infra work is great.", Timestamp: base.AddDate(0, 6, 0)},
		{Content: "Big news: I left Stripe and I'm at Google now, on the search infra team.", Timestamp: base.AddDate(1, 0, 0)},
	}

	const iters = 3
	var synD, extD []time.Duration
	for i := 0; i < iters; i++ {
		t0 := time.Now()
		gist, err := p.Synthesize(context.Background(), episodes)
		d := time.Since(t0)
		if err != nil {
			t.Fatalf("iter %d Synthesize: %v", i, err)
		}
		synD = append(synD, d)
		t.Logf("iter %d Synthesize %.2fs -> %s", i, d.Seconds(), gist)

		t0 = time.Now()
		triples, err := p.ExtractTriples(context.Background(), gist)
		d = time.Since(t0)
		if err != nil {
			t.Fatalf("iter %d ExtractTriples: %v", i, err)
		}
		extD = append(extD, d)
		t.Logf("iter %d ExtractTriples %.2fs -> %d triples", i, d.Seconds(), len(triples))
		for _, tr := range triples {
			t.Logf("    (%s | %s | %s) conf=%.2f", tr.Subject, tr.Predicate, tr.Object, tr.Confidence)
		}
		if len(triples) == 0 {
			t.Errorf("iter %d: zero triples from a gist with clear facts", i)
		}
	}

	t.Logf("MEASURED median seconds/call: Synthesize %.2f, ExtractTriples %.2f",
		median(synD).Seconds(), median(extD).Seconds())
}

func median(ds []time.Duration) time.Duration {
	s := append([]time.Duration{}, ds...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	return s[len(s)/2]
}
