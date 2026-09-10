# Memora — Continuum Memory Architecture

A Go memory service for LLM agents — episodic ingest → vector + graph stores → DBSCAN
consolidation → reranked, budget-constrained context assembly — **and an eval harness
built to falsify its own design claims.**

The second half is the point. This repo started from a paper (`paper.tex`) whose headline
benchmark numbers it could not support. Rather than keep them, the work since has been to
measure what the code actually does. Several of the architecture's most attractive ideas
**did not survive that measurement**, and this README leads with those.

> **On `paper.tex`.** Its LOCOMO comparison table (85.4% accuracy, 88.4% temporal, 75.6%
> multi-hop, <1% hallucination, "beats full-context GPT-4-Turbo") is sourced in the paper's
> own words to *"aggregated data from 2025-2026 evaluations"* — external literature
> figures, **not a run of this code**. No test or eval here produces a LOCOMO score. The
> paper is kept for the design it describes; treat none of its numbers as results of this
> repository.

---

## What was measured, and what it cost

Every number below comes from a tracked test in this repo. Commands are in
[Running the evals](#running-the-evals).

### The pre-registered forgetting experiment: hypothesis falsified

The registration (`cma/eval/PREREGISTRATION.md`) was committed **before any experiment
code existed**. H1 was that θ-based archival improves retrieval. The full gated run
(`TestForgettingExperiment`) **falsified it on all five registered clauses**:

| clause | required | measured |
| --- | --- | --- |
| effect | ≥ +10 pp | **−2.22 pp** (wrong sign) |
| significance | p < 0.05 | **McNemar p = 1.0** |
| cost | ≤ 2 pp control loss | **−3.02 pp** |
| mechanism | beat random archival | **SR@1 0.4493 ≤ 0.5143** random max, 20 replicates |
| rival | beat a recency prior | **0.4493 ≤ 0.7429** (D3) |

Corpus: 500 instances, 948 sessions, 10,960 turns, counts re-derived in Go
(`TestRegisteredCorpusCounts`). The Go test is *designed to fail its own assertion* once
H1 falls — the failure is the finding.

`cmd/probe` then re-derives the nine design numbers the registration rested on, from the
sha256-verified corpus and the real embedder, and **strikes the three that don't
reproduce** by the pre-registered rule rather than asterisking them:

| number | recorded | measured | |
| --- | --- | --- | --- |
| `lexical_top1_rate` | 0.2143 | **0.3286** | struck |
| `ku_strict_n` | 49 | **48** | struck |
| `truncated_at_256_frac` | 0.449 | **0.4369** | struck |

### Three retrieval ideas that didn't work

| arm | result |
| --- | --- |
| **RRF fusion** (BM25 ⊕ cosine, k=60) | **worse than cosine alone** — recall@1 97/100 vs 98/100, MRR 0.9850 vs 0.9900 |
| **Neo4j 2-hop hybrid** | **structurally inert** — identical top-10 order to cosine on 110/110 queries |
| **DIG heuristic reranker** | **exactly neutral** — identical top-10 to raw cosine on 110/110 queries |

The graph arm's inertness has a diagnosed root cause, logged at the call site: the dedup
key in `service.go` builds `"ep:" + Episode.ID` where that ID is empty for graph results,
so every graph fact for a query collides on one key, and graph candidates score 0.15
against the *worst* vector candidate's 0.34 — they can never enter the top 10.

Baseline for context: BM25 recall@1 92/100 (MRR 0.9517), cosine 98/100 (MRR 0.9900).

### Component measurements that did hold

- **DBSCAN** recovers planted cluster structure at production dimension: **4/4 clusters,
  sizes [8 8 8 8], 0 noise** at 1536-d (`clustering_test.go`).
- **Greedy vs exact knapsack**: over 30 seeded 18-item instances the greedy Lagrangian
  allocator reaches **mean 0.9689** of the exact 0/1-DP optimum, **worst 0.8457**, and hits
  the optimum **10/30**. It approximates; it does not solve.
- **A real over-budget defect**: force-include can breach the token budget by **7.32×**
  (`TestForceIncludeBreachThreshold`). Found by its own test, not in production.
- **Local ONNX embeddings, no API key and no network**: all-MiniLM-L6-v2 (384-d, quint8)
  behind a hand-rolled WordPiece tokenizer. cos(cat, feline paraphrase) = **0.5323** vs
  cos(cat, unrelated) = **0.0359**.

## Known dead paths

Documented and replaced rather than left to look alive:

- **The surprisal segmenter never fired.** `internal/llm/openai.go` requests logprobs at
  `MaxTokens: 1`, so it retained 1 character of an ~11,884-character input; with the cap
  lifted it produced **0 boundaries across 15,227 words**, and its "tokens" were
  whitespace words — there is no tokenizer in `go.mod`. It is out of the ingest path,
  replaced by deterministic sentence packing (`internal/segmentation/structural.go`),
  which round-trips 2,000 words into 7 episodes with 100% word preservation.
- **DIG's logprob scorer is unreachable** — no configured provider exposes the logprobs
  surface it needs. The shipped reranker is the heuristic above, and it carries no
  surprisal term because that value is now a constant 0.
- **There is no forgetting.** `DeleteByIDs` has **zero production callers**, and
  `decay_factor` is a single non-compounding write. Nothing in the shipped read or write
  path evicts, archives or expires a memory.

Two defects are flagged by their own tests and left visible: `Cluster()` output order is
non-deterministic (collected from a Go map without sorting), and `computeCentroid` panics
on mixed-dimension embeddings.

## Stack

Go 1.22 + Gin · Qdrant (episodic vectors, gRPC) · Neo4j 5.18 (semantic graph, bi-temporal
`valid_to` filtering) · Redis + Asynq (consolidation worker) · Prometheus · ONNX Runtime
(local embeddings) · a Claude-CLI provider for triple extraction · Next.js 14 + TypeScript
landing page in `web/`.

## Layout

```
cma/                   the Go backend
  cmd/api              Gin server
  cmd/probe            re-derives and strikes the pre-registration's numbers
  cmd/consolidate      LLM triple extraction over clustered episodes
  internal/            ingest, retrieval, consolidation, vectorstore, graphstore,
                       llm, segmentation, dig, knapsack, metrics
  eval/                the harness: BM25 / cosine / RRF / live-graph / DIG arms,
                       PREREGISTRATION.md, and frozen artifacts in eval/out/
  README.md            backend doc — PREDATES the Aug-31 work and is stale in places
web/                   Next.js landing page (scaffold; not wired to the backend)
paper.tex              the original paper — see the note above
```

## Running the evals

```sh
cd cma
go test ./internal/...                      # no network, no Docker, no API key
docker compose up -d qdrant neo4j redis     # the eval arms need Qdrant; graph arms need Neo4j
go test ./eval/...
MEMORA_RUN_FORGET_EXPERIMENT=1 go test ./eval/ -run TestForgettingExperiment
```

**18 tracked test files, 74 test functions.** `./internal/...` is green with no external
dependencies. The forgetting experiment is gated behind the env var and asserts its own
falsified hypothesis, so it fails by design.
