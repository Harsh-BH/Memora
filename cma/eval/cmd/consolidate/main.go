// Command consolidate runs the SHIPPED consolidation (Sleep) cycle end to end
// over the LongMemEval knowledge-update subset -- the same 70 KU-permissive
// instances / 1640 turns the falsified forgetting experiment used -- and
// freezes every extracted triple to an immutable artifact.
//
// Nothing here is a re-implementation of the pipeline. It ingests episodes,
// then calls consolidation.Worker.ProcessTask, which is the production handler:
// GetUnconsolidated -> DBSCAN -> Synthesize -> ExtractTriples ->
// FindConflicts/ResolveConflict/InsertTriple -> MarkConsolidated -> UpdateDecay.
// The only thing wrapped is the llm.Provider, by a tee that records what the
// model returned. worker.go, conflict.go and neo4j.go are untouched.
//
// FREEZE RULE: extraction runs ONCE. The artifact this writes is the input to
// every downstream arm; nothing may re-extract after seeing a result, or the
// pre-registration is void and the extractor becomes a tunable knob.
//
//	go run ./eval/cmd/consolidate -dry-run          # cluster + cost only, no LLM
//	go run ./eval/cmd/consolidate                   # the real, once-only run
//
// TOP-UP: the freeze rule forbids re-extracting a cluster that already
// produced triples; it does NOT forbid finishing clusters that never got an
// answer. A pass whose CLI calls died (spend limit, network) leaves those
// episodes still consolidation_status=pending in Qdrant, so -resume -only
// picks up exactly them -- no already-consolidated episode is touched, so
// nothing is re-extracted -- and -merge folds the result into the previous
// artifact with both passes recorded in the header.
//
//	go run ./eval/cmd/consolidate -resume -only a,b,c \
//	    -merge eval/out/consolidation_triples.pass1.json \
//	    -out   eval/out/consolidation_triples.json
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/memora/cma/configs"
	"github.com/memora/cma/eval"
	"github.com/memora/cma/internal/consolidation"
	"github.com/memora/cma/internal/graphstore"
	"github.com/memora/cma/internal/llm"
	"github.com/memora/cma/internal/metrics"
	"github.com/memora/cma/internal/models"
	"github.com/memora/cma/internal/vectorstore"
)

// Registered embedder configuration (PREREGISTRATION.md 4.9). Same values the
// forgetting experiment used, so the cached vectors are reused verbatim and the
// clustering this run sees is the clustering that experiment's D0 arm saw.
const (
	embedMaxSeqLen = 256
	embedDim       = 384
)

// options is every knob this command has. It is a struct rather than a
// parameter list because the top-up flags took run() past ten arguments.
type options struct {
	dryRun     bool
	outPath    string
	logPath    string
	collection string
	model      string
	par        int
	nInst      int
	maxRounds  int
	only       string
	merge      string
	resume     bool
}

func main() {
	var o options
	flag.BoolVar(&o.dryRun, "dry-run", false, "cluster and cost only; no LLM call, no artifact")
	flag.StringVar(&o.outPath, "out", filepath.Join("eval", "out", "consolidation_triples.json"), "frozen artifact path")
	flag.StringVar(&o.logPath, "log", filepath.Join("eval", "out", "consolidation_run.log"), "run log path")
	flag.StringVar(&o.collection, "collection", "cma_eval_consolidate", "Qdrant collection")
	flag.StringVar(&o.model, "model", "", "claude CLI --model (empty = CLI default)")
	flag.IntVar(&o.par, "par", 4, "instances consolidated in parallel; also the CLI concurrency cap")
	flag.IntVar(&o.nInst, "instances", 0, "limit instances (0 = all 70; anything else is a SMOKE run)")
	flag.IntVar(&o.maxRounds, "max-rounds", 3, "ProcessTask rounds per user before giving up")
	flag.StringVar(&o.only, "only", "", "comma-separated user_ids to consolidate (top-up); requires -resume")
	flag.StringVar(&o.merge, "merge", "", "artifact from a previous pass to fold this pass into")
	flag.BoolVar(&o.resume, "resume", false, "keep the existing Qdrant collection and its consolidation_status; do not re-ingest")
	flag.Parse()

	if err := run(o); err != nil {
		fmt.Fprintf(os.Stderr, "FAILED: %v\n", err)
		os.Exit(1)
	}
}

func run(o options) error {
	ctx := context.Background()
	start := time.Now()

	// --- corpus, sha-verified (V8) ---
	all, err := eval.LoadLongMemEval(eval.LongMemEvalPath())
	if err != nil {
		return err
	}
	ku := eval.Select(all, eval.IsKUPermissive)
	if len(ku) != 70 {
		return fmt.Errorf("KU-permissive = %d, registered 70", len(ku))
	}
	registered := len(ku)
	smoke := o.nInst > 0 && o.nInst < len(ku)
	if smoke {
		ku = ku[:o.nInst]
	}
	// -only: a top-up pass over named instances. Without -resume this would
	// drop the collection and re-ingest, which resets every OTHER instance to
	// pending and throws away the pass whose gap we are here to close.
	var only []string
	if o.only != "" {
		if !o.resume {
			return fmt.Errorf("-only without -resume would drop %s and reset the instances it does not name; pass -resume", o.collection)
		}
		want := map[string]bool{}
		for _, id := range strings.Split(o.only, ",") {
			if id = strings.TrimSpace(id); id != "" {
				want[id] = true
			}
		}
		var kept []*eval.LMEInstance
		for _, in := range ku {
			if want[in.QuestionID] {
				kept = append(kept, in)
				only = append(only, in.QuestionID)
				delete(want, in.QuestionID)
			}
		}
		if len(want) > 0 {
			var missing []string
			for id := range want {
				missing = append(missing, id)
			}
			sort.Strings(missing)
			return fmt.Errorf("-only names %d id(s) that are not KU-permissive instances: %s", len(missing), strings.Join(missing, ","))
		}
		ku = kept
		fmt.Printf("top-up: %d of %d instances (%s)\n", len(ku), registered, strings.Join(only, ","))
	}

	// --- embedder + the experiment's own vector cache ---
	root := "third_party" // run from cma/
	modelFile := filepath.Join(root, "models", "all-MiniLM-L6-v2", "model_quint8_avx2.onnx")
	embedder, err := llm.NewLocalEmbedProvider(
		filepath.Join(root, "onnxruntime", "lib", "libonnxruntime.so"),
		modelFile,
		filepath.Join(root, "models", "all-MiniLM-L6-v2", "vocab.txt"),
		embedMaxSeqLen, embedDim)
	if err != nil {
		return fmt.Errorf("embedder: %w", err)
	}
	defer embedder.Close()
	modelSHA, err := eval.FileSHA256(modelFile)
	if err != nil {
		return err
	}
	cachePath := filepath.Join(eval.DatasetDir(), fmt.Sprintf("embed_cache_minilm%d_%d.gob", embedDim, embedMaxSeqLen))
	cache := eval.LoadEmbedCache(cachePath, modelSHA, embedMaxSeqLen, embedDim)

	// --- Qdrant: a dedicated collection, dropped and rebuilt ---
	store, err := vectorstore.NewQdrantStore(configs.QdrantConfig{
		Host: "localhost", GRPCPort: 6334, Collection: o.collection,
		VectorSize: embedDim, HnswM: 16, HnswEF: 100, WaitForIndex: true,
	})
	if err != nil {
		return fmt.Errorf("qdrant: %w", err)
	}
	defer store.Close()
	if !o.resume {
		dropCollection(o.collection)
	}
	if err := store.EnsureCollection(ctx); err != nil {
		return fmt.Errorf("ensure collection: %w", err)
	}

	// --- ingest: turns in 4.4 order, one user per instance ---
	// Under -resume the collection already holds these episodes WITH their
	// consolidation_status, and that status is the resume point: re-upserting
	// would stamp every episode back to pending and re-extract clusters the
	// freeze rule says are done. So count the turns and touch nothing.
	embedded := 0
	turnsByUser := map[string]int{}
	pendingBefore := map[string]int{}
	for _, in := range ku {
		turns, err := in.Turns()
		if err != nil {
			return err
		}
		turnsByUser[in.QuestionID] = len(turns)
		if o.resume {
			n, err := store.CountUnconsolidated(ctx, in.QuestionID)
			if err != nil {
				return fmt.Errorf("%s: count pending: %w", in.QuestionID, err)
			}
			pendingBefore[in.QuestionID] = n
			continue
		}
		eps := make([]models.Episode, 0, len(turns))
		for _, tn := range turns {
			vec, ok := cache.Get(tn.Content)
			if !ok {
				if vec, err = embedder.Embed(ctx, tn.Content); err != nil {
					return fmt.Errorf("embed %s: %w", tn.ID, err)
				}
				cache.Put(tn.Content, vec)
				embedded++
			}
			eps = append(eps, models.Episode{
				ID: tn.ID, UserID: in.QuestionID, Content: tn.Content,
				Embedding: vec, Timestamp: tn.Timestamp(), EventID: in.QuestionID,
				MemoryType:          models.MemoryEpisodic,
				ConsolidationStatus: models.StatusPending,
				DecayFactor:         1.0,
				AssociatedEntities:  []string{},
			})
		}
		if err := store.Upsert(ctx, eps); err != nil {
			return fmt.Errorf("%s: upsert: %w", in.QuestionID, err)
		}
	}
	if embedded > 0 {
		if err := cache.Save(cachePath); err != nil {
			return err
		}
	}
	totalTurns := 0
	for _, n := range turnsByUser {
		totalTurns += n
	}
	if o.resume {
		stillPending := 0
		for _, n := range pendingBefore {
			stillPending += n
		}
		fmt.Printf("resumed: %d turns over %d instances, %d still pending (nothing re-ingested)\n", totalTurns, len(ku), stillPending)
	} else {
		fmt.Printf("ingested %d turns over %d instances (%d newly embedded)\n", totalTurns, len(ku), embedded)
	}

	// --- dry run: what the shipped DBSCAN will actually ask the LLM for ---
	clusterer := consolidation.NewDBSCAN(0.3, 3)
	if o.dryRun {
		return costReport(ctx, store, ku, clusterer)
	}

	// --- the real run ---
	logFile, err := os.Create(o.logPath)
	if err != nil {
		return err
	}
	defer logFile.Close()
	slog.SetDefault(slog.New(slog.NewJSONHandler(io.MultiWriter(logFile, os.Stderr), &slog.HandlerOptions{Level: slog.LevelInfo})))

	cliCfg := configs.LLMConfig{Provider: "claude-cli", Model: o.model, TimeoutSeconds: 300, MaxConcurrency: o.par}
	cli, err := llm.NewClaudeCLIProvider(cliCfg)
	if err != nil {
		return err
	}
	streamPath := o.outPath + ".stream.jsonl"
	stream, err := os.Create(streamPath)
	if err != nil {
		return err
	}
	defer stream.Close()
	rec := &recorder{inner: cli, byGist: map[string]*clusterRecord{}, stream: stream}

	graphDB, err := graphstore.NewNeo4jStore(configs.Neo4jConfig{
		URI: "bolt://localhost:7687", Username: "neo4j", Password: "cmapassword", Database: "neo4j",
	})
	if err != nil {
		return fmt.Errorf("neo4j: %w", err)
	}
	defer graphDB.Close(ctx)

	m := metrics.New()
	worker := consolidation.NewWorker(store, rec, clusterer,
		consolidation.NewConflictResolver(graphDB, 0.95),
		configs.ConsolidationConfig{DecayRate: 0.95, MaxUnconsolidated: 10, InactivityTimeout: 15 * time.Minute},
		m)

	llmStart := time.Now()
	results := make([]userResult, len(ku))
	sem := make(chan struct{}, o.par)
	var wg sync.WaitGroup
	for i, in := range ku {
		wg.Add(1)
		go func(i int, userID string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			r := userResult{UserID: userID, Turns: turnsByUser[userID], Passes: []int{1}}
			for r.Rounds = 0; r.Rounds < o.maxRounds; {
				n, err := store.CountUnconsolidated(ctx, userID)
				if err != nil {
					r.Err = err.Error()
					break
				}
				r.Remaining = n
				if n == 0 {
					break
				}
				task, err := consolidation.NewConsolidateTask(userID)
				if err != nil {
					r.Err = err.Error()
					break
				}
				r.Rounds++
				if err := worker.ProcessTask(ctx, task); err != nil {
					r.Err = err.Error()
					break
				}
				if n2, err := store.CountUnconsolidated(ctx, userID); err == nil {
					r.Remaining = n2
					if n2 == n {
						// no progress: every cluster failed, another round
						// would only repeat it.
						break
					}
				}
			}
			results[i] = r
			fmt.Printf("[%d/%d] %s rounds=%d pending_after=%d %s\n", i+1, len(ku), userID, r.Rounds, r.Remaining, r.Err)
		}(i, in.QuestionID)
	}
	wg.Wait()
	llmWall := time.Since(llmStart)

	// --- freeze ---
	records := rec.records()
	triples := 0
	for _, r := range records {
		triples += len(r.Triples)
	}
	head, _ := os.ReadFile(filepath.Join("..", ".git", "HEAD"))

	this := passInfo{
		Pass:          1,
		StartedUTC:    start.UTC().Format(time.RFC3339),
		FinishedUTC:   time.Now().UTC().Format(time.RFC3339),
		CLIVersion:    cliVersion(),
		ModelAlias:    o.model,
		ModelResolved: resolvedModel(o.model),
		Instances:     instanceIDs(ku),
		Clusters:      len(records),
		Triples:       triples,
		CLICalls:      rec.calls,
		CLIFailures:   rec.failures,
		WallClock:     llmWall.String(),
		Why:           "first pass: all registered instances",
	}
	art := artifact{
		Kind:          "memora/consolidation-triples/v1",
		Frozen:        "Extraction ran ONCE per cluster. This file is the immutable input to every downstream arm; nothing may re-extract a cluster that already has triples.",
		RunUTC:        this.StartedUTC,
		RunEndUTC:     this.FinishedUTC,
		Smoke:         smoke,
		CLIVersion:    this.CLIVersion,
		Model:         o.model,
		ResolvedModel: this.ModelResolved,
		ModelNote:     "Model is the --model alias passed to the CLI; ResolvedModel is the concrete model id the CLI reported for that alias, measured by one --output-format json probe in this same process.",
		CLIFlags:      cliCfg,
		Corpus:        "LongMemEval lme_oracle.json sha256=" + eval.SHA256LongMemEval + ", KU-permissive rule (PREREGISTRATION.md 4.2)",
		Instances:     len(ku),
		Turns:         totalTurns,
		EmbedModelSHA: modelSHA,
		DBSCAN:        "epsilon=0.3 minPoints=3 (the shipped configs/config.yaml values)",
		RepoHEAD:      string(head),
		SynthPrompt:   llm.SynthesizePromptTemplate,
		ExtractPrompt: llm.ExtractTriplesPromptTemplate,
		LLMWallClock:  llmWall.String(),
		CLICalls:      rec.calls,
		CLIFailures:   rec.failures,
		TotalTriples:  triples,
		Clusters:      len(records),
		Users:         results,
		Records:       records,
		Passes:        []passInfo{this},
	}
	for _, r := range records {
		r.Pass = 1
	}
	if o.merge != "" {
		if err := mergeInto(&art, o.merge); err != nil {
			return fmt.Errorf("merge %s: %w", o.merge, err)
		}
	}

	blob, err := json.MarshalIndent(art, "", "  ")
	if err != nil {
		return err
	}
	blob = append(blob, '\n')
	if err := writeFrozen(o.outPath, blob); err != nil {
		return err
	}
	sum := sha256.Sum256(blob)
	digest := hex.EncodeToString(sum[:])
	if err := writeFrozen(o.outPath+".sha256", []byte(digest+"  "+filepath.Base(o.outPath)+"\n")); err != nil {
		return err
	}

	fmt.Printf("\n=== FROZEN ===\nartifact %s\nsha256   %s\npasses   %d\nclusters %d  triples %d  cli_calls %d  cli_failures %d\nllm wall %s  total wall %s\n",
		o.outPath, digest, len(art.Passes), art.Clusters, art.TotalTriples, art.CLICalls, art.CLIFailures,
		llmWall.Round(time.Second), time.Since(start).Round(time.Second))
	for _, u := range art.Users {
		if u.Remaining > 0 {
			fmt.Printf("STILL INCOMPLETE %s pending_after=%d %s\n", u.UserID, u.Remaining, u.Err)
		}
	}
	return nil
}

// writeFrozen replaces a path that a previous freeze left mode 0444. os.WriteFile
// cannot open a read-only file, so the old one is unlinked first -- deliberately,
// and only here, so overwriting a frozen artifact is never accidental.
func writeFrozen(path string, blob []byte) error {
	if _, err := os.Stat(path); err == nil {
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	return os.WriteFile(path, blob, 0o444)
}

func instanceIDs(ku []*eval.LMEInstance) []string {
	ids := make([]string, 0, len(ku))
	for _, in := range ku {
		ids = append(ids, in.QuestionID)
	}
	sort.Strings(ids)
	return ids
}

// mergeInto folds a previous pass's artifact underneath this one: the earlier
// records are kept verbatim (they are frozen), this pass's are appended, and the
// header grows a passes[] that names every pass, so a two-pass artifact can
// never be read as a single run.
func mergeInto(art *artifact, prevPath string) error {
	blob, err := os.ReadFile(prevPath)
	if err != nil {
		return err
	}
	var prev artifact
	if err := json.Unmarshal(blob, &prev); err != nil {
		return err
	}
	sum := sha256.Sum256(blob)

	prevPasses := prev.Passes
	if len(prevPasses) == 0 {
		// A pre-passes[] artifact: reconstruct its pass entry from its header.
		prevPasses = []passInfo{{
			Pass: 1, StartedUTC: prev.RunUTC, FinishedUTC: prev.RunEndUTC,
			CLIVersion: prev.CLIVersion, ModelAlias: prev.Model, ModelResolved: prev.ResolvedModel,
			Instances: userIDs(prev.Users), Clusters: prev.Clusters, Triples: prev.TotalTriples,
			CLICalls: prev.CLICalls, CLIFailures: prev.CLIFailures, WallClock: prev.LLMWallClock,
			Why: "first pass",
		}}
	}
	n := len(prevPasses) + 1
	this := art.Passes[0]
	this.Pass = n
	this.Why = fmt.Sprintf("top-up pass %d: instances the previous pass left with unconsolidated episodes", n)

	for _, r := range prev.Records {
		if r.Pass == 0 {
			r.Pass = 1
		}
	}
	for _, r := range art.Records {
		r.Pass = n
	}

	// Users: this pass's entry supersedes the earlier one for the same id, but
	// carries the earlier rounds and pass list forward.
	// Index by position, not by pointer: the append below can move the backing
	// array, and a stale *userResult would silently drop a merge.
	at := map[string]int{}
	merged := append([]userResult{}, prev.Users...)
	for i := range merged {
		if len(merged[i].Passes) == 0 {
			merged[i].Passes = []int{1}
		}
		at[merged[i].UserID] = i
	}
	for _, u := range art.Users {
		i, ok := at[u.UserID]
		if !ok {
			u.Passes = []int{n}
			at[u.UserID] = len(merged)
			merged = append(merged, u)
			continue
		}
		merged[i].Rounds += u.Rounds
		merged[i].Remaining = u.Remaining
		merged[i].Err = u.Err
		merged[i].Passes = append(merged[i].Passes, n)
	}
	turns := 0
	for _, u := range merged {
		turns += u.Turns
	}

	art.Passes = append(prevPasses, this)
	art.MergedFrom = fmt.Sprintf("%s sha256=%s", filepath.Base(prevPath), hex.EncodeToString(sum[:]))
	art.Provenance = fmt.Sprintf("PRODUCED IN %d PASSES, NOT ONE RUN. Pass 1 was starved mid-flight: its `claude -p` calls "+
		"began failing with exit status 1 once the account hit its spend limit, so %d clusters never got a gist and the "+
		"instances holding them were frozen with unconsolidated episodes. The later pass(es) re-ran ONLY those instances, "+
		"resuming from the consolidation_status left in Qdrant, so no cluster was extracted twice. See passes[] for each "+
		"pass's timestamps, model, CLI version and instance list; records[].pass says which pass produced each record.",
		n, prev.CLIFailures)
	art.Records = append(prev.Records, art.Records...)
	art.Users = merged
	art.Instances = len(merged)
	art.Turns = turns
	art.Clusters = len(art.Records)
	art.CLICalls += prev.CLICalls
	art.CLIFailures += prev.CLIFailures
	art.TotalTriples = 0
	for _, r := range art.Records {
		art.TotalTriples += len(r.Triples)
	}
	art.RunUTC = prevPasses[0].StartedUTC
	art.Smoke = art.Smoke || prev.Smoke
	if d1, err1 := time.ParseDuration(prev.LLMWallClock); err1 == nil {
		if d2, err2 := time.ParseDuration(art.LLMWallClock); err2 == nil {
			art.LLMWallClock = (d1 + d2).String() + " (summed over passes)"
		}
	}
	return nil
}

func userIDs(us []userResult) []string {
	ids := make([]string, 0, len(us))
	for _, u := range us {
		ids = append(ids, u.UserID)
	}
	sort.Strings(ids)
	return ids
}

// costReport prints exactly how many Synthesize/ExtractTriples calls the
// shipped clusterer will demand, before any of them is paid for.
func costReport(ctx context.Context, store *vectorstore.QdrantStore, ku []*eval.LMEInstance, c *consolidation.DBSCAN) error {
	sizes := map[int]int{}
	clusters, multi := 0, 0
	for _, in := range ku {
		eps, err := store.GetUnconsolidated(ctx, in.QuestionID, 100)
		if err != nil {
			return err
		}
		for _, cl := range c.Cluster(eps) {
			clusters++
			sizes[len(cl.Episodes)]++
			if len(cl.Episodes) > 1 {
				multi++
			}
		}
	}
	var keys []int
	for k := range sizes {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	fmt.Printf("clusters=%d (multi-episode=%d, singleton=%d)\n", clusters, multi, clusters-multi)
	for _, k := range keys {
		fmt.Printf("  size %2d: %d clusters\n", k, sizes[k])
	}
	fmt.Printf("CLI calls needed = 2 x %d = %d\n", clusters, 2*clusters)
	return nil
}

// --- the recording tee ---

// clusterRecord is one cluster's gist and the triples extracted from it. Seq is
// the call order WITHIN a pass, so (Pass, Seq) is the unique key in a merged
// artifact, not Seq alone.
type clusterRecord struct {
	Pass       int             `json:"pass"`
	Seq        int             `json:"seq"`
	UserID     string          `json:"user_id"`
	EpisodeIDs []string        `json:"episode_ids"`
	Gist       string          `json:"gist"`
	Triples    []models.Triple `json:"triples"`
}

// recorder is an llm.Provider that delegates everything and keeps what came
// back. It is the ONLY thing between worker.go and the real provider.
type recorder struct {
	inner    llm.Provider
	mu       sync.Mutex
	seq      int
	calls    int
	failures int
	byGist   map[string]*clusterRecord
	orphans  []*clusterRecord
	// stream is append-only crash insurance: a completed record is on disk
	// before the next one starts, so a death at cluster 700 does not cost 700
	// clusters of extraction that the freeze rule forbids re-running.
	stream *os.File
}

func (r *recorder) Synthesize(ctx context.Context, eps []models.Episode) (string, error) {
	gist, err := r.inner.Synthesize(ctx, eps)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	if err != nil {
		r.failures++
		return gist, err
	}
	ids := make([]string, 0, len(eps))
	for _, e := range eps {
		ids = append(ids, e.ID)
	}
	r.seq++
	rec := &clusterRecord{Seq: r.seq, UserID: eps[0].UserID, EpisodeIDs: ids, Gist: gist}
	if prev, ok := r.byGist[gist]; ok {
		// identical gist from two clusters: keep both, join the newest.
		r.orphans = append(r.orphans, prev)
	}
	r.byGist[gist] = rec
	return gist, nil
}

func (r *recorder) ExtractTriples(ctx context.Context, content string) ([]models.Triple, error) {
	triples, err := r.inner.ExtractTriples(ctx, content)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	if err != nil {
		r.failures++
		return triples, err
	}
	rec, ok := r.byGist[content]
	if !ok {
		r.seq++
		rec = &clusterRecord{Seq: r.seq, Gist: content}
		r.orphans = append(r.orphans, rec)
	}
	rec.Triples = triples
	if r.stream != nil {
		if line, err := json.Marshal(rec); err == nil {
			r.stream.Write(append(line, '\n'))
			r.stream.Sync()
		}
	}
	return triples, nil
}

// records returns every cluster that produced a gist, in call order.
func (r *recorder) records() []*clusterRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := append([]*clusterRecord{}, r.orphans...)
	for _, rec := range r.byGist {
		out = append(out, rec)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Seq < out[j].Seq })
	return out
}

func (r *recorder) Generate(ctx context.Context, p string) (string, error) {
	return r.inner.Generate(ctx, p)
}
func (r *recorder) Embed(ctx context.Context, t string) ([]float32, error) {
	return r.inner.Embed(ctx, t)
}
func (r *recorder) EmbedBatch(ctx context.Context, t []string) ([][]float32, error) {
	return r.inner.EmbedBatch(ctx, t)
}
func (r *recorder) CountTokens(t string) int { return r.inner.CountTokens(t) }
func (r *recorder) GetTokenProbabilities(ctx context.Context, t string) ([]llm.TokenProb, error) {
	return r.inner.GetTokenProbabilities(ctx, t)
}
func (r *recorder) ScoreDIG(ctx context.Context, q, d string) (float64, error) {
	return r.inner.ScoreDIG(ctx, q, d)
}

var _ llm.Provider = (*recorder)(nil)

// --- artifact ---

// userResult is one instance's outcome. Remaining > 0 means the instance was
// frozen with episodes still consolidation_status=pending -- i.e. incomplete.
type userResult struct {
	UserID    string `json:"user_id"`
	Turns     int    `json:"turns"`
	Rounds    int    `json:"rounds"`
	Remaining int    `json:"pending_after"`
	Err       string `json:"error,omitempty"`
	Passes    []int  `json:"passes,omitempty"`
}

// passInfo records one extraction pass. A merged artifact carries one per pass
// so the header can never present a two-pass artifact as a single run.
type passInfo struct {
	Pass          int      `json:"pass"`
	StartedUTC    string   `json:"started_utc"`
	FinishedUTC   string   `json:"finished_utc"`
	CLIVersion    string   `json:"claude_cli_version"`
	ModelAlias    string   `json:"model_alias"`
	ModelResolved string   `json:"model_resolved"`
	Instances     []string `json:"instances"`
	Clusters      int      `json:"clusters_with_a_gist"`
	Triples       int      `json:"triples"`
	CLICalls      int      `json:"cli_calls"`
	CLIFailures   int      `json:"cli_failures"`
	WallClock     string   `json:"llm_wall_clock"`
	Why           string   `json:"why"`
}

type artifact struct {
	Kind          string            `json:"kind"`
	Frozen        string            `json:"freeze_rule"`
	Provenance    string            `json:"provenance,omitempty"`
	MergedFrom    string            `json:"merged_from,omitempty"`
	Passes        []passInfo        `json:"passes"`
	RunUTC        string            `json:"run_started_utc"`
	RunEndUTC     string            `json:"run_finished_utc"`
	Smoke         bool              `json:"smoke_run"`
	CLIVersion    string            `json:"claude_cli_version"`
	Model         string            `json:"model_alias"`
	ResolvedModel string            `json:"model_resolved"`
	ModelNote     string            `json:"model_note"`
	CLIFlags      configs.LLMConfig `json:"llm_config"`
	Corpus        string            `json:"corpus"`
	Instances     int               `json:"instances"`
	Turns         int               `json:"turns"`
	EmbedModelSHA string            `json:"embed_model_sha256"`
	DBSCAN        string            `json:"clusterer"`
	RepoHEAD      string            `json:"repo_head"`
	SynthPrompt   string            `json:"synthesize_prompt_template"`
	ExtractPrompt string            `json:"extract_triples_prompt_template"`
	LLMWallClock  string            `json:"llm_wall_clock"`
	CLICalls      int               `json:"cli_calls"`
	CLIFailures   int               `json:"cli_failures"`
	TotalTriples  int               `json:"total_triples"`
	Clusters      int               `json:"clusters_with_a_gist"`
	Users         []userResult      `json:"users"`
	Records       []*clusterRecord  `json:"records"`
}

// dropCollection removes a leftover collection over Qdrant's HTTP API -- the
// gRPC client here has no delete, and a leaked collection walks the container
// into its nofile limit.
func dropCollection(name string) {
	req, err := http.NewRequest(http.MethodDelete, "http://localhost:6333/collections/"+name, nil)
	if err != nil {
		return
	}
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err == nil {
		resp.Body.Close()
	}
}

// resolvedModel asks the CLI which concrete model an alias maps to, so the
// artifact header names the model that actually ran rather than the alias.
func resolvedModel(alias string) string {
	args := []string{"-p", "--strict-mcp-config", "--mcp-config", `{"mcpServers":{}}`,
		"--no-session-persistence", "--output-format", "json"}
	if alias != "" {
		args = append(args, "--model", alias)
	}
	cmd := exec.Command(cliBin(), args...)
	cmd.Dir = os.TempDir()
	cmd.Stdin = strings.NewReader("Reply with only the word OK.")
	out, err := cmd.Output()
	if err != nil {
		return "unknown: " + err.Error()
	}
	var probe struct {
		ModelUsage map[string]struct {
			CanonicalModel string `json:"canonicalModel"`
		} `json:"modelUsage"`
	}
	if err := json.Unmarshal(out, &probe); err != nil {
		return "unknown: " + err.Error()
	}
	var ids []string
	for k, v := range probe.ModelUsage {
		ids = append(ids, k+" (canonical "+v.CanonicalModel+")")
	}
	sort.Strings(ids)
	return strings.Join(ids, ", ")
}

func cliVersion() string {
	out, err := exec.Command(cliBin(), "--version").Output()
	if err != nil {
		return "unknown: " + err.Error()
	}
	return strings.TrimSpace(string(out))
}

func cliBin() string {
	if b := os.Getenv("CLAUDE_CLI_BIN"); b != "" {
		return b
	}
	return "claude"
}
