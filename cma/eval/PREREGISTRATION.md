# PRE-REGISTRATION — "Does forgetting stale facts improve retrieval?"

**Registered:** 2026-09-01. **Repo head at registration:** `8ee224e`.
**Status:** committed **before** any experiment code exists and before any SR@1 value has been
computed for any arm. Nothing in §1–§5 may change after the first arm number is produced.
Amendments are additive, dated, and must appear below §7 with the reason; nothing may be deleted.

## Evidence labelling, enforced throughout

| Label | Meaning |
|---|---|
| **MEASURED-HERE** | Re-derived by the registrant on 2026-09-01 against the real files / real source. The command or `path:line` is given inline. |
| **PROVISIONAL-UNREPRODUCED** | Reported by the prior design pass but **not** re-derived; its probe files were deleted (Gap 10). May not be cited in any write-up until `cma/eval/cmd/probe/main.go` (§4.10) reproduces it. |
| **PROJECTED** | Arithmetic on labelled inputs. Not a result. |

---

## 1. THE PRIMARY HYPOTHESIS AND THE PRIMARY CELL

### 1.1 H1 — one hypothesis, falsifiable

> **H1.** On the LongMemEval knowledge-update subset (KU-permissive, n = 70), archiving every turn
> that is the nearest strictly-older neighbour of some later turn under IDF-weighted Jaccard
> similarity ≥ θ\* **raises SR@1 by ≥ +10 absolute points over RAW**, **at a cost of ≤ 2 absolute
> points of recall@5** on the 398 non-abstention control queries, **and by more than a
> per-instance rate-matched RANDOM-ARCHIVE control**.

All three clauses must hold. Any one failing falsifies H1 (§5.1).

### 1.2 The primary cell — exactly one

**Arm D2, at the single θ\* selected by the control-set rate rule (§4.6), scored as SR@1 on
KU-permissive n = 70, tested against RAW by exact two-sided McNemar at α = 0.05.**

That is **one** statistical test. It is the only test in this experiment that carries confirmatory
weight. Every other arm, every other grid cell, KU-strict, the pooled row, and LoCoMo are
**EXPLORATORY** and are labelled as such on every line where they appear (Gap 19).

**Why D2 and not D1 or D0.** The prior probe measured lexical top-1 pairing at 21.4 % against dense
top-1 at 7.1 % and shipped-DBSCAN pair coverage at 8.6 % (all **PROVISIONAL-UNREPRODUCED**). D2 is
therefore the pre-registered favourite on *mechanism* grounds, named before any outcome number
exists. **If `cmd/probe` fails to reproduce the lexical > dense ordering, the primary arm does not
change** — it was registered — but this justification is struck from the record and the write-up
must say so. Reversing the choice after the probe fails would be selection on the outcome.

**Deviation from the design plan, declared:** the plan made KU-**strict** (n = 49) primary. It is
demoted to EXPLORATORY here. The strict rule ("numeric-token containment … else answer-tokens-minus-
question-tokens overlap strictly greater and ≥ 0.50") has several unfrozen sub-choices, its output ID
list was never committed, and its n = 49 is **PROVISIONAL-UNREPRODUCED**. KU-permissive is a
two-clause rule with no free parameter that the registrant re-derived from the file today
(**MEASURED-HERE**: n = 70), contains **0** abstention items (**MEASURED-HERE**), and has more power.
A primary cell must be reproducible from the registration alone.

---

## 2. ARMS

One retrieval pass drives every arm. Retrieve **once** per query at depth **D = 200** over the
unarchived corpus, producing ranked list `R`. Each arm is the offline set-filter
`R_X = [r ∈ R : r ∉ A_X][:K]`, `K = 50`, where `A_X` is that arm's archive set. Pre-flight assert
PF4 (§4.11) licenses this by checking it against the real `Search` + `MustNot` path.

| # | Arm | Archive set `A` | Role | Weight |
|---|---|---|---|---|
| **RAW** | shipped system | ∅ | control; flat cosine | baseline of the primary cell |
| **D2** | lexical nearest-older(θ\*) | §4.5 | **the primary arm** | **CONFIRMATORY** |
| **D1** | dense nearest-older(θ) | §4.5 | mechanism without DBSCAN's minPts trap | EXPLORATORY |
| **D0** | shipped consolidator | `consolidation.NewDBSCAN(0.3, 3).Cluster` per instance, clusters with ≥ 2 members, archive all but the newest | tests the **as-shipped config** | EXPLORATORY |
| **D3** | global recency prior (rival) | ∅ — rescore `R` as `cos + w·exp(−Δdays/τ)` | the cheap rival, oracle-tuned in its own favour | EXPLORATORY |
| **RANDOM-ARCHIVE** | rate-matched random | §2.1 | **the control that separates "detection works" from "archiving anything helps"** | **mandatory clause of H1** |
| **ORACLE** | perfect forgetting | exactly the labelled stale-gold turns | ceiling; SR@1 = 1.0 **by arithmetic**, never a result | **harness self-check V2** |
| **ANTI-ORACLE** | inverted gold | exactly the labelled **current**-gold turns | **the only check that the harness reads the gold map in the right orientation** | **harness self-check V1** |
| **FLOOR** | archive-all-older | every turn not in the newest session | degenerate exploit; proves the joint rule is load-bearing | **harness self-check V7** |

**Exactly these nine arms. No arm may be added, removed, or redefined after the first number is
computed** — including the "obvious" arm that a surprising result seems to call for. A new arm
suggested by a result is a new experiment with its own registration.

**The reranker is OFF in every arm.** Ranking is the raw Qdrant cosine score. Justification is
arithmetic, not stylistic: `dig.go:98` adds `0.3·exp(−age_h/24)`, identically 0.000000 on a
2023-dated corpus, and `dig.go:104` multiplies by `DecayFactor`, 1.0 everywhere (all
**MEASURED-HERE** at those exact lines). DIG collapses to cosine. See PF1–PF3 (§4.11) for what
replaces the plan's degenerate "Rerank order == cosine order" assert.

### 2.1 RANDOM-ARCHIVE — the non-negotiable rate-matched control (Gap 22)

Without it, a positive D2 result cannot distinguish *"supersession detection works"* from
*"archiving ~4 % of older turns helps at all."* FLOOR only shows the degenerate extreme.

- **Rate match is per instance, exact:** for every instance `i`, `|A_random(i)| = |A_D2*(i)|`.
  This matches not merely the global rate but its distribution across instances.
- **Eligibility set:** uniform without replacement from
  `{turns having ≥ 1 strictly-later turn in the same instance}` — i.e. every turn except the last in
  the total order (§4.4). **Reason:** D1/D2 structurally *cannot* archive the newest turn. Sampling
  from all turns would let RANDOM archive the current gold at a rate the detector cannot, biasing
  the control downward and flattering D2. Matching the eligibility set is the conservative choice.
- **Replicates:** R = 20, seeds `20260901 + r` for `r ∈ 0..19` (§4.8). Report the full distribution,
  not a single draw.
- **Registered comparison:** the H1 mechanism clause requires
  `SR@1(D2*) > max over the 20 replicates`. Max-of-20 is a one-sided permutation-style threshold at
  p ≈ 1/21 = 0.048, exact, no distributional assumption.

### 2.2 ANTI-ORACLE — the orientation check (Gap 23)

`ORACLE = 1.0` is arithmetic and **would pass identically if the stale and current columns of the
gold map were swapped**. It proves nothing about orientation. ANTI-ORACLE archives the *current*
gold; the first surviving gold must then be a stale turn, so:

> **Registered assertion: SR@1(ANTI-ORACLE) = 0.0 exactly.** Any other value ⇒ **VOID** (V1).

**Vacuity guard, registered:** SR@1 is undefined when no gold survives, and a mean over an empty set
would report 0.0 for the wrong reason. ANTI-ORACLE's defined-query count must be
**≥ 0.5 × RAW's defined-query count**, else the check is inconclusive and the run is VOID.

### 2.3 D3 landing below RAW (Gap 21)

A recency prior can hurt. If `SR@1(D3 best cell) < SR@1(RAW)`, "beats D3" is vacuous. Registered
now: the rival clause is
`SR@1(D2*) > max( SR@1(D3 best cell), SR@1(RAW) + 0.10 )` — the rival comparison can never be
*easier* than the RAW comparison. If D3's best cell lands below RAW, the write-up must state:
*"D3's best cell fell below RAW; the rival-beating clause was satisfied vacuously and carries no
evidential weight."*

D3 grid: `w ∈ {0.05, 0.1, 0.3} × τ ∈ {30, 90, 365}` days, Δ measured from `question_date` to the
turn's timestamp. **All 9 cells reported.** D3 is deliberately allowed to select its own best cell on
the outcome metric — it is the rival, and handicapping ourselves against it is the point.

---

## 3. THE METRIC — deterministic, no LLM, no judge, no generation

For KU query `q` with frozen gold stale set `S_q` and current set `C_q`:

```
first_gold(q, X) = the first r in R_X with r ∈ S_q ∪ C_q
SR@1(q, X) = 1   if first_gold ∈ C_q
           = 0   if first_gold ∈ S_q
           = ⊥   if no gold survives in R_X       (excluded from the mean, counted, reported)
SR@1(X)    = mean over q where SR@1 is defined
```

Comparison of Qdrant point UUIDs against a committed CSV. Integer set membership. It cannot be
inflated by unit length, chunk size, or candidate count, because every arm shares one index, one
embedding pass, one query-vector set, and one ranked list.

**Control falsifier metric:** recall@5 and MRR over the 398 non-abstention control queries, computed
by the same arithmetic already in `eval/hard_eval_test.go:243-245`.

### 3.1 The arithmetic identity that governs interpretation — registered now, not discovered later

KU-permissive instances hold **20 to 24 turns** (**MEASURED-HERE**: min 20, max 24, p50 24, total
1640 over 70 instances). With `D = 200` and `K = 50`, `R_X` contains **every unarchived turn** — no
truncation occurs in the primary row. Therefore:

> **In the primary row, `SR@1(q, X)` depends only on which of `q`'s own gold turns are in `A_X`.
> Archiving a non-gold turn cannot change SR@1.**

Four consequences, all registered before any run:

1. **RANDOM-ARCHIVE's expected Δ is `P(random draw hits a gold turn) × effect`, i.e. ≈ 0.** A
   near-null RANDOM-ARCHIVE is a **pass**, not a failure of the control. Its job is to be the thing
   D2 must beat, and D2's margin over it is the mechanism evidence.
2. **Pair-catch rate and ΔSR@1 are arithmetically linked, not independent evidence.** They must
   never be reported as two corroborating findings.
3. **SR@1 is structurally blind to collateral damage.** This is precisely why the 407/398-query
   control falsifier is joint and mandatory, and why SR@1 may never be quoted without control
   recall@5 beside it.
4. **The pooled row (§4.13) is the only row where non-gold archiving can move the metric** (10,960
   turns against K = 50 truncates), and therefore the only row where RANDOM-ARCHIVE could plausibly
   be non-null.

### 3.2 Mandatory per-query attribution (Gap 24)

A 2×2 table, reported for D2\* and for every arm with a non-zero archive set:

| | own stale gold ∈ A | own stale gold ∉ A |
|---|---|---|
| **SR@1 = 1** | n | n |
| **SR@1 = 0** | n | n |

Given §3.1, the right-hand column must be **identical to RAW** unless the detector archived one of
that query's *current* gold turns. Any right-column cell that differs from RAW without a current-gold
archive is a harness bug and voids the run.

### 3.3 Diagnostics reported on every arm

pair-catch rate; archive precision `|A ∩ gold-stale| / |A|`; archive recall
`|A ∩ gold-stale| / |gold-stale|`; archived-point count **queried back from Qdrant** (§4.12), never
read off a clusterer log; evidence-found rate (the ⊥ rate); the both-golds-retrieved subset and its
SR@1 separately; same-session vs cross-session split of `A`. **No number in this experiment may be
read off Prometheus** — `worker.go:82-85` and `:110` increment `ConsolidationRuns` and
`ClustersFormed` in a deferred block *before* the first failure, so the metrics claim consolidation
ran and formed clusters while `EpisodesConsolidated` stays 0.

---

## 4. EVERYTHING FIXED NOW — thresholds, rates, seeds, corpora

### 4.1 Datasets — durable location, URLs, licences (Gap 5, Gap 29)

`/tmp` on this box is **tmpfs 7.7 G, RAM-backed** (**MEASURED-HERE**: `df -h /tmp`), so a reboot
deletes anything registered there. Both datasets were copied to the real filesystem **before this
document was committed**:

**Canonical path: `~/data/memora-eval/`** on `/dev/nvme0n1p7` (175 G, 18 G available,
**MEASURED-HERE**). Total 18.2 MB. `SHA256SUMS` is present in that directory.

| File | Bytes | sha256 (**MEASURED-HERE**, `sha256sum`) | Source | Licence |
|---|---|---|---|---|
| `lme_oracle.json` | 15,388,478 | `821a2034d219ab45846873dd14c14f12cfe7776e73527a483f9dac095d38620c` | `huggingface.co/datasets/xiaowu0162/longmemeval-cleaned` (public, keyless) | MIT |
| `locomo10.json` | 2,805,274 | `79fa87e90f04081343b8c8debecb80a9a6842b76a7aa537dc9fdf651ea698ff4` | `raw.githubusercontent.com/snap-research/locomo/main/data/locomo10.json` | **CC BY-NC 4.0** |
| `LICENSE-LoCoMo.txt` | 19,347 | `41003d4a74749c0220e33dd415042164b5a1093ed401f36277234f772d22d3d0` | `raw.githubusercontent.com/snap-research/locomo/main/LICENSE.txt` (HTTP 200, **MEASURED-HERE**) | header reads `Attribution-NonCommercial 4.0 International` |

The data files are **not** committed to git — 18 MB of third-party corpora in a repo is avoidable and
the shas pin identity exactly. What *is* committed alongside this file is
`cma/eval/testdata/DATASETS.md`, recording path + URL + sha256 + licence for each, and the harness
**must** verify both shas at startup and `t.Fatal` on mismatch.

**LoCoMo publication gate, registered:** no LoCoMo number may be published until (a)
`LICENSE-LoCoMo.txt` is present on disk, (b) the CC BY-NC 4.0 attribution line is in the write-up,
(c) LoCoMo is used for **research only, never any product claim**, and (d) the sha256 of the
**stripped** subset is published *first*. The loader must `delete` `observation`, `session_summary`,
and `event_summary` from every conversation object at parse time — enforced in code, not prose.
Those fields are GPT-written gists tagged with **the same `dia_id`s the gold evidence points to**;
reading any of them is a guaranteed, meaningless win.

### 4.2 Query sets — rules, not lists, so they are reproducible from this document

All **MEASURED-HERE** by re-parsing `lme_oracle.json` on 2026-09-01. 500 instances; 948 sessions;
10,960 turns; question types: temporal-reasoning 133, multi-session 133, knowledge-update 78,
single-session-user 70, single-session-assistant 56, single-session-preference 30.

| Set | Rule (exact) | n |
|---|---|---|
| **KU-permissive** — the primary set | `question_type == "knowledge-update"` **and** for each of the 2 `answer_session_ids`, the corresponding haystack session contains ≥ 1 turn with `has_answer == true` | **70** |
| Control | `question_type != "knowledge-update"` and ≥ 1 turn anywhere with `has_answer == true` | **407** |
| **Control-primary** — the falsifier set | Control **minus** the 9 `_abs` items | **398** |
| KU-strict — EXPLORATORY | the plan's audit rule, contingent on `cmd/probe` | **49**, PROVISIONAL-UNREPRODUCED |

Registered structural facts (**MEASURED-HERE**), each of which the harness must assert:
every KU instance has exactly 2 sessions and exactly 2 `answer_session_ids`; all 70 KU-permissive
instances have 2 **distinct** `haystack_dates`; all 142 `has_answer` turns in KU-permissive carry
`role == "user"`; KU-permissive contains **0** `_abs` items. The plan's alternative permissive rule
("exactly 2 `has_answer` flags") yields **68** (**MEASURED-HERE**) and is **not** the registered rule.

### 4.3 The 9 abstention instances (Gap 18)

**MEASURED-HERE:** the corpus contains 30 `question_id`s ending `_abs`; **9 of them sit inside the
407-query control set** (12 multi-session, 6 temporal, 6 KU, 6 single-session-user across the whole
file). Their answers are of the form *"The information provided is not enough. You mentioned fixing
the fence but did not mention purchasing cows from Peter."* For those queries, **retrieving the
flagged turn is not the goal** — the task is abstention. 9/407 = 2.2 % against a 2 pp falsifier
threshold: the choice can flip the falsifier by itself, which is why it is fixed now.

> **Registered:** the H1 falsifier is computed on the **398 non-abstention control queries**. The 9
> `_abs` queries are reported as a **separate, labelled EXPLORATORY row** with their own recall@5,
> and carry **no** confirm/falsify weight. KU-permissive contains none of them, so the primary cell
> is unaffected.

### 4.4 The total order over turns — the tie-break substrate

Every "older / newer" statement in this document resolves against one strict total order per
instance:

```
key(turn) = ( parse(haystack_dates[i]), i, turn_idx )      # session date, haystack index, position
```

lexicographic, ascending. Comparison is on this key, **never on a float timestamp**, so timestamp
collisions cannot introduce ambiguity. The key is strict: within a session `turn_idx` is unique, and
`i` disambiguates same-dated sessions.

`Episode.Timestamp = parse(haystack_dates[i]) + turn_idx minutes` is written for the record and for
D3's Δdays; it is not the ordering authority.

### 4.5 "Nearest-older" — fully pinned (Gap 16)

For a turn `t`, let `U(t)` = turns strictly preceding `t` in the §4.4 order. If `U(t)` is empty, `t`
contributes nothing.

- **D2 (primary).** Tokenize with `eval/bm25.go:13 tokenizeSimple` (lowercase; split on any
  non-letter, non-digit rune). Build a `NewBM25` index over that instance's turns; `idf(term) =
  log(1 + (n − df + 0.5)/(df + 0.5))` (`eval/bm25.go:73`, **MEASURED-HERE** at that line), `n` = the
  instance's turn count. With token **sets** `A`, `B` (set, not multiset):
  `simIDF(a,b) = Σ_{τ ∈ A∩B} idf(τ) / Σ_{τ ∈ A∪B} idf(τ)`.
  `u* = argmax_{u ∈ U(t)} simIDF(t,u)`. **Archive `u*` iff `simIDF(t,u*) ≥ θ`.**
- **D1.** Identical, with `d(t,u) = 1 − cos` over the frozen 384-d vectors;
  `u* = argmin_{u ∈ U(t)} d(t,u)`. **Archive `u*` iff `d(t,u*) ≤ θ`.**
- **Sign convention, stated so nobody flips it later:** D1 thresholds a **distance** with `≤`;
  D2 thresholds a **similarity** with `≥`.
- **Tie-break:** on equal score, take the `u` **earliest** in the §4.4 order. The order is strict, so
  no second tie-break can be needed.
- **Same-session neighbours are allowed.** No cross-session requirement. **Reason:** requiring a
  different session would build LongMemEval's 2-session KU structure into the detector — corpus-
  specific tuning that would not transfer. The same-session / cross-session split of `A` is reported
  as a diagnostic instead.
- **No cascade.** `A` is computed in **one pass** over the frozen episode list, with all pairwise
  scores taken against the **full unarchived** corpus. A turn already in `A` may still be selected as
  another turn's nearest-older (`A` is a set; it is not added twice) and may still serve as an anchor
  `t`. **Reason:** an iterative scheme's output depends on evaluation order, reintroducing exactly
  the order-dependence that forces §4.9's sorted ingest.
- **No role filter.** All turns, user and assistant, are eligible as both `t` and `u`. A role filter
  is a free knob.

**Honest framing, registered:** D1 and D2 are episode-level reinventions of the repo's *designed*
supersession mechanism. `ConflictResolver.ResolveAndInsert` (`conflict.go:42`) →
`FindConflicts` (`neo4j.go:241`) → `ResolveConflict` (`neo4j.go:328`, which closes `valid_to` and
multiplies confidence server-side) is exactly "detect that a newer fact supersedes an older one." It
has zero tests and its only caller (`worker.go:141`) is unreachable behind `ExtractTriples`. The
write-up must say this rather than presenting D1/D2 as novel (Gap 1).

### 4.6 Grids and the archive-rate rule — why 4.39 %, not 10 % (Gap 17)

- **D0:** fixed at the shipped `dbscan_epsilon: 0.3`, `dbscan_min_points: 3`
  (`configs/config.yaml:52-53`, **MEASURED-HERE**). One cell. Mandatory `len(Episodes) >= 2` guard:
  `clustering.go:107-109` returns every noise point as a singleton cluster. **Do not lower epsilon
  toward 1.0** — `computeCentroid` sizes its accumulator from `episodes[0]` (`clustering.go:166`)
  then indexes it with a longer episode's range; verified index-out-of-range at eps 1.0 / minPts 1.
- **D1:** θ ∈ {0.20, 0.30, 0.40, 0.50}. **D2:** θ ∈ {0.10, 0.15, 0.20, 0.25}. **All cells reported.**

**The target rate ρ\* is fixed at 0.0439, and it is not a taste.** It is the **ORACLE arm's own
archive rate**, computed from the frozen gold map and published here before any arm runs:
72 stale-gold turns ÷ 1640 total turns over the 70 KU-permissive instances = **0.0439**
(**MEASURED-HERE**; 72 stale and 70 current gold turns — two instances carry two stale golds). It is
the rate at which supersession *actually occurs* on the only part of this corpus where supersession
is labelled. The plan's "nearest 10 % of turns" had no derivation and was written after the probe;
it is replaced.

> **θ\* selection rule.** θ\* is the grid cell whose **archive rate over the 407 control instances**
> is nearest `ρ* = 0.0439`. The control instances contain **no KU item, no supersession label, and no
> outcome metric**, so the selection cannot see SR@1. Control instances average 21.6 turns (8,804
> turns / 407 instances, **MEASURED-HERE**) against KU-permissive's 23.4, so the two rates are
> comparable without rescaling.
>
> **Interpretation, stated plainly:** every archive on the control set is by construction unlabelled
> — calibrating the detector's firing rate there to the phenomenon's true base rate is an explicit
> false-positive budget equal to the base rate.
>
> **Stated objection, not hidden:** ρ\* is derived from the gold map. It uses only the map's
> *marginal count*, never any pair's identity, and the map is a registered, published artifact fixed
> before any arm runs. θ\* is then chosen on data containing none of those pairs.
>
> **Ties:** if two cells are equidistant from ρ\*, take the one with the **lower** archive rate —
> conservative, fewer archives, less collateral risk on the falsifier.
>
> **Monotonicity and interiority are not assumed.** The full control-set archive-rate curve is
> reported for every cell. **If the nearest cell is a grid endpoint, that is reported as "the target
> rate lies outside the registered grid" and the endpoint remains θ\*. The grid is NOT extended.**
> Extending a grid after seeing rates is the forking path this document exists to close.
>
> **Do not pick θ from the measured twin-distance median.** That is tuning on the gold pairs.

### 4.7 IDF corpus — per-instance (Gap 15)

> **Registered: per-instance.** For a KU instance, the IDF corpus is that instance's ~24 turns. For
> the pooled row (§4.13) only, IDF is recomputed over the 10,960 pooled turns.

**Reasons, in order.** (1) The primary arm's memory *is* per-instance (`user_id = question_id`), so
the detector must see exactly what a per-user deployment would see; a pooled IDF is information from
other users' memories that no such system has. (2) It keeps the detector's input identical between
the primary row and any per-instance re-run. (3) It is the smaller computation.
**Registered limitation:** IDF over ~24 documents is statistically thin. `bm25.go:73`'s `+1`
smoothing keeps every value non-negative (**MEASURED-HERE** at that line), so the metric is
well-defined, but the discrimination is weak and that is a validity limit of D2, stated up front.

### 4.8 Seeds

The only randomness in the experiment is RANDOM-ARCHIVE. Base seed **20260901** (the registration
date). Replicate `r ∈ 0..19` uses `rand.New(rand.NewSource(20260901 + int64(r)))`. Instances are
iterated in ascending `question_id` byte order; within an instance the eligible turn list is taken
in §4.4 order, `rand.Shuffle`d, and the first `|A_D2*(i)|` entries taken. Embedding is deterministic
at batch size 1; DBSCAN is deterministic given §4.9's sorted ingest. **No other seed exists.**

### 4.9 Embedder, ingest order, and the collection

- `maxSeqLen = 256`, dim **384**, **batch size 1 throughout**, vectors computed **once** and shared
  byte-identically by every arm. Batch size 1 is mandatory and asserted: `local_embed.go:100-110`
  documents that this quantized model derives its int8 activation scale from whole-batch statistics,
  so the same text embeds to ~0.99 cosine of itself depending on batch composition — the same order
  as the score gaps that decide ranking.
- **Registered truncation limitation:** 44.9 % of turns exceed 256 wordpieces
  (**PROVISIONAL-UNREPRODUCED**), and the answer-bearing span sits at char offset p50 194 / max 526
  (**PROVISIONAL-UNREPRODUCED**). Truncation is uniform across arms — a **validity limit, not a
  confound**, because every arm reads the same vectors.
- **Ingest order** sorted by the §4.4 key. **Reason:** `clustering.go:82-84` flips a border point's
  noise label to the current cluster on first touch, so unsorted ingestion makes D0 unreproducible
  and invites re-rolling until a favourable clustering appears.
- **Collection (Gap 14):** `cma_eval_forget`, dim **384**, `Distance_Cosine`, created by the harness
  and dropped in `t.Cleanup`. `configs/config.yaml:11 vector_size: 1536` (**MEASURED-HERE**) is
  **not** read by this harness. The production `cma_episodes` collection is **never** touched.
- **Metrics registry (Gap 7):** the harness **must** call the existing `sharedMetrics()`
  (`eval/shared_test.go:50`, **MEASURED-HERE**). A fresh `metrics.New()` in a new test file panics
  the package binary on the promauto global registry.

### 4.10 The re-derivation script — no §0 number survives without it (Gap 10)

The measurement that chose this design is unreproducible by its own admission ("the probe files were
deleted after use"). Registered resolution:

> **`cma/eval/cmd/probe/main.go` — committed, deterministic, no flags that change its output — must
> re-derive every §0 number and write `cma/eval/out/probe.json` plus the frozen artifacts.**

Its required outputs: `d(stale gold, current gold)` percentiles; `d(current gold, nearest non-gold
turn)` percentiles; stale-twin top-1 rate under dense and under IDF-weighted Jaccard; gold-pair
coverage at `eps = 0.3`; the count of instances with a false neighbour inside 0.30; the flat-cosine
current-preference proxy; the embedder throughput table; the KU-strict ID list; and the frozen gold
map CSV.

> **Registered rule: any number labelled PROVISIONAL-UNREPRODUCED in this document may not be cited
> in any write-up until `cmd/probe` reproduces it. If the script's value differs from the recorded
> value, BOTH are printed side by side with the delta, and the script's value is authoritative. If
> the script cannot reproduce a number at all, that number is STRUCK from the record** — including
> the 47.1 % flat-cosine proxy, the 0.514 / 0.217 medians, the 8.6 % / 60-of-70 eps-0.3 figures, and
> the 7.1 % / 21.4 % detector top-1 rates. Deleting an unreproducible number is a successful outcome
> of this rule, not a failure of the experiment.

### 4.11 Pre-flight asserts — replacing the degenerate one (Gap 12)

The plan's assert "`dig.Rerank` order == cosine order on 20 sampled queries" is degenerate and can
pass for the wrong reason. Episodes built directly bypass `models.NewEpisode` (`models.go:64` is the
only writer of `DecayFactor = 1.0`, **MEASURED-HERE**), so `DecayFactor` is the zero value **0.0**;
`dig.go:104 score *= result.Episode.DecayFactor` (**MEASURED-HERE** at that line) then makes every
score **exactly 0.0**; `min_score: -0.5` (`config.yaml:46`) keeps them all; and `dig.go:75` is
`sort.Slice`, **not** `sort.SliceStable` (**MEASURED-HERE**). "Order preserved" is then a coin flip.

Registered replacements — each voids the run on failure:

- **PF0 — the trap is disarmed at the source.** Every Episode the harness constructs sets
  `DecayFactor = 1.0`, `ImportanceScore = 0.0`, `ConsolidationStatus = StatusPending` **explicitly**,
  never by zero value; a 20-point sample is round-tripped through Qdrant and asserted to read back
  `DecayFactor == 1.0`.
- **PF1 — a check that cannot pass for the wrong reason.** `dig.Rerank` over 5 synthetic candidates
  with **distinct** cosine scores (0.9, 0.7, 0.5, 0.3, 0.1), **identical** `Timestamp`,
  `ImportanceScore = 0`, `DecayFactor = 1.0` must return strictly descending cosine order. Distinct
  scores make `sort.Slice`'s instability irrelevant; identical timestamps make `dig.go:98` a shared
  constant.
- **PF2 — the trap recorded as a runnable check.** One candidate with `DecayFactor = 0.0` must score
  **exactly 0.0**, documenting `dig.go:104`'s annihilation instead of tripping over it.
- **PF3 — normalisation.** Upsert `v` and `2v`; assert identical `Distance_Cosine` scores.
- **PF4 — production-path equivalence.** For 20 sampled queries, real `Search` with the new
  `MustNot consolidation_status == "archived"` must return **exactly** the offline-computed `R_X`.
  **This is the assert that licenses the entire offline simulation.** Failure ⇒ VOID (V5).

**Missing-key scope note (Gap 27):** `QdrantStore.Upsert`'s payload map always writes
`consolidation_status`, and `cma_eval_forget` is created fresh by the harness, so a point with no
such key cannot exist in this experiment. No test is written for that case; the invariant is recorded
instead of built.

### 4.12 Counting archived points without a new interface method (Gap 6, Gap 26)

The only Count in the repo is `CountUnconsolidated` (`qdrant.go:329`) with `status=pending`
hardcoded. Registered resolution — **no production interface change**:

- **G1's archived count** comes from a raw `POST localhost:6333/collections/cma_eval_forget/points/count`
  with the `consolidation_status == "archived"` filter, from the harness, via `net/http`.
- **The collection drop** (`t.Cleanup`) is a raw `DELETE localhost:6333/collections/{name}`.
  **Not optional:** `shared_test.go:8-38` documents that collections are never dropped, `cma_qdrant`
  ships `RLIMIT_NOFILE=1024`, and ~3 accumulated collections make Qdrant refuse connections on both
  ports with what looks like a network error. Recovery is `docker restart cma_qdrant` plus a
  `curl -X DELETE` — **restart, never recreate**: the `cma_qdrant_data` volume and the production
  `cma_episodes` collection must survive.
- **`ArchiveByIDs` goes on `*QdrantStore`, NOT on the `VectorStore` interface.** `QdrantStore` is the
  only implementation, there are no mocks, and `eval/hard_eval_test.go:92` already holds the concrete
  `vectorstore.NewQdrantStore(...)` return (**MEASURED-HERE**). This is a declared deviation from the
  plan's step A2.
- `consolidation_status` is **already** a keyword payload index (`qdrant.go:89-95`,
  **MEASURED-HERE**), so the `MustNot` filter needs no index change.

### 4.13 The pooled secondary row — its own detector pass and its own gate (Gap 13, Gap 20)

The pooled row re-upserts the same 10,960 turns under `user_id = "pool"`. `QdrantStore.Upsert`
rewrites the **whole** payload map (`qdrant.go:130-141`) and has no archive keys, so re-upserting
would **silently reset `consolidation_status`**. Registered:

> The pooled row uses a **separate collection** (`cma_eval_forget_pooled`) and a separate `user_id`,
> built from the cached vectors, with **its own detector pass** (per §4.7, pooled IDF) and **its own
> `ArchiveByIDs` application**. The order is always **upsert-all → detect → archive → search**, never
> archive → upsert. The primary collection is never re-upserted after archiving.

**Gate:** the pooled row is **EXPLORATORY**. Its registered outputs are SR@1 on KU-permissive under
the pooled index **plus its own evidence-found rate** (which will not be 1.0 — K = 50 against 10,960
turns truncates). It carries **no** confirm/falsify weight: at n = 70 over a differently-truncated
retrieval task it cannot bear inferential weight, and saying so now prevents it being promoted to
evidence later.

**LoCoMo control gate:** metric = recall@5 over LoCoMo cat2 (temporal, n = 321) against gold
`dia_id`s, arms RAW vs D2 at θ\*; registered threshold **drop ≤ 2 pp**, the same bound as the primary
falsifier because it is the same kind of claim ("does not damage recall where there is nothing to
forget"). **EXPLORATORY, no p-value:** LoCoMo's questions nest inside 10 conversations, so the
effective n is 10, not 321, and an item-level test would overstate significance by more than an order
of magnitude. Gated behind §4.1's licence + stripped-sha rule.

### 4.14 Statistics — one confirmatory test (Gap 19)

- **Primary:** exact two-sided McNemar via the binomial tail, D2\* vs RAW, SR@1, KU-permissive
  n = 70, α = 0.05. **One test. No correction is applied because there is one test.**
- **Everything else:** EXPLORATORY. Discordant-pair counts printed **raw** beside every p-value; all
  exploratory p-values labelled descriptive; **the total count of exploratory tests is printed** so a
  reader can apply their own correction. No exploratory p-value may appear in a claim.
- **Equivalence bound: ±10 pp** (Wilson). Arithmetic, not taste: half-width
  `1.96·√(b+c)/n ≤ 0.10` needs `b+c ≤ (0.10·70/1.96)² = 12.7`, i.e. ≤ 12 discordant pairs —
  achievable. `±3 pp` needs `b+c ≤ 1.15`, i.e. ≤ 1 discordant pair — arithmetically unreachable, and
  its use would silently degrade to the bare `p > 0.05` this project forbids.
- **Power, computed and stated at its weakest point.** At the PROVISIONAL 21.4 % lexical catch rate
  and a RAW SR@1 near 0.47, discordant pairs ≈ `0.214 × 0.53 × 70 ≈ 8`, essentially all in one
  direction (archiving a stale gold can only move SR@1 up); exact binomial `p = 2 × 0.5⁸ = 0.0078`.
  **But ΔSR@1 = 8/70 = 11.4 pp against a registered threshold of +10 pp — a margin of 1.4 pp.**
  Registered plainly: **H1's effect threshold is calibrated at the edge of its own projection, and a
  modest shortfall in catch rate falsifies it.** That is the point of a falsifiable hypothesis; it is
  said here rather than discovered afterwards. The design is **not** powered to distinguish a
  +5 pp effect from noise.
- **Uncertainty is over items only.** LongMemEval instances are independent (no LoCoMo-style nesting),
  but the archive set is one **deterministic** function of the corpus, so there is no sampling
  variance to bootstrap over. n = 70 supports a large effect or a located null, nothing finer.

### 4.15 Decay, and the two safety fixes that land before any run

`decay_rate` stays at the shipped **0.95** (`config.yaml:54`). **Archival is boolean and applies no
score haircut** — decay and forgetting are separate mechanisms and are not blended.

- **`configs/config.go applyDefaults` has no `DecayRate` default.** If `decay_rate` is ever absent,
  `UpdateDecay` writes `decay_factor = 0.0` to every consolidated episode and `dig.go:104` annihilates
  cosine, recency and importance on the vector path — the graph-arm bug reproduced verbatim. Only the
  literal `0.95` at `config.yaml:54` holds it off. **The clamp `conflict.go:33-35` already has lands
  before any run.**
- **`Wait: ptr(true)` changes live ingest latency (Gap 9).** `qdrant.go:151-163`'s BUG comment
  explicitly says to evaluate that cost first. **Registered:** the wall-clock of the existing
  `retrieval_eval` ingest is measured **before and after** the change and both numbers reported in
  the commit message. **Rollback rule: a > 2× regression on that number means `Wait` becomes a
  constructor option rather than an unconditional flag.** Without `Wait`, an archive-then-verify
  measures indexing lag rather than forgetting.

---

## 5. STOPPING RULES

### 5.1 What FALSIFIES H1

Evaluated on the primary cell only. Any **one** of these falsifies H1; the write-up must name which
clause failed:

| | Clause | Falsified when |
|---|---|---|
| F1 | effect | `SR@1(D2*) − SR@1(RAW) < +10 pp` on KU-permissive n = 70 |
| F2 | significance | exact two-sided McNemar `p ≥ 0.05` |
| F3 | cost | control recall@5 on the **398** drops by `> 2 pp` |
| F4 | mechanism | `SR@1(D2*) ≤ max` over the 20 RANDOM-ARCHIVE replicates |
| F5 | rival | `SR@1(D2*) ≤ max( SR@1(D3 best cell), SR@1(RAW) + 0.10 )` |

**A falsified H1 is a result, not a failure.** Four located nulls, each of which must be reported by
name rather than as a shrug:

- **G1 fails (nothing archived):** *"D2 archives nothing at θ\*."* For D0 this outcome is PROJECTED
  and is itself the headline finding about the shipped configuration — at `eps = 0.3` a distance rule
  reaches a small minority of gold pairs and `min_points = 3` cannot form a cluster from a 2-point
  pair at all, so the shipped consolidator's supersession recall is structurally zero.
- **G1 passes, G2 fails (pair-catch = 0):** the null is located exactly at *"the detector fires, but
  lexical/embedding proximity does not identify supersession."* This is a real, publishable result:
  it **locates the blocker at fact-level semantic identity — precisely what `llm.ExtractTriples` was
  supposed to supply and cannot.**
- **F1/F2 fail with G1 and G2 passing:** the mechanism works and the effect is too small; report the
  ±10 pp equivalence interval, never a bare `p > 0.05`.
- **F3 fails while F1 passes:** *"forgetting buys freshness with recall."* That is **not** the
  thesis's claim and is reported as a failure. FLOOR shows what that failure looks like at its
  extreme.

### 5.2 What VOIDS the experiment

A void run says nothing about H1. Every void and every re-run is reported with its reason. **A failed
gate is a reported result about the detector, never a licence to re-roll θ.**

| | Condition | Why it voids |
|---|---|---|
| **V1** | `SR@1(ANTI-ORACLE) ≠ 0.0` exactly, **or** its defined-query count `< 0.5 ×` RAW's | the harness reads the gold map in the wrong orientation, or the check is vacuous over an empty set (Gap 23) |
| **V2** | `SR@1(ORACLE) ≠ 1.0` exactly | the gold map or the archive filter is wrong |
| **V3** | RAW evidence-found rate `< 0.95` on the primary cell | §3.1 proves it must be 1.0 (20–24 turns, K = 50); anything less means the index or query set is broken (Gap 25) |
| **V4** | `SR@1(RAW) ∉ [0.35, 0.65]` | **the premise gate** (Gap 25, Gap 11) — see below |
| **V5** | PF4 fails: real `Search` + `MustNot` ≠ offline `R_X` on 20 sampled queries | the offline simulation is not licensed |
| **V6** | G1: Qdrant-queried `Count(consolidation_status="archived") == 0` for the primary arm | nothing was manipulated; reported as a finding about D2 at θ\*, not as a null about H1 |
| **V7** | `SR@1(FLOOR) ≠ 1.0` exactly | archiving does not actually remove points from retrieval — the mechanism under test is inert |
| **V8** | either dataset sha256 mismatches §4.1 | the corpus is not the registered corpus |
| **V9** | §3.2's right-hand column differs from RAW without a current-gold archive | harness bug (§3.1 makes it arithmetically impossible otherwise) |

**V4, the premise gate, with its reasoning.** The registered effect threshold is +10 pp absolute. For
that to be a meaningful fraction of the ceiling, headroom `1 − SR@1(RAW)` must be ≥ 0.35, giving
`RAW ≤ 0.65`. **If RAW lands near ceiling the experiment is void** — at RAW = 0.85 a +10 pp claim
would consume two thirds of the remaining headroom and is a different, easier test than the one
registered. The lower bound closes Gap 11's missing reconciliation: the probe's flat-cosine proxy was
47.1 % (**PROVISIONAL-UNREPRODUCED**) from an ad-hoc within-instance scorer, while RAW comes from
Qdrant HNSW at depth 200. If `RAW < 0.35` the harness and the probe disagree about the corpus by
> 12 pp, and that disagreement must be reconciled and reported **before** any arm is compared.
Either way, **`SR@1(RAW)` is reported as a finding in its own right.**

Two further arithmetic self-checks the harness must run, failure of either being V9-class:
`SR@1(ANTI-ORACLE) = 0` and `SR@1(ORACLE) = 1` as above, and **`control recall@5(ORACLE) ==
control recall@5(RAW)` exactly** — ORACLE's archive set is defined only on KU instances, so it must
be a no-op on the control set.

### 5.3 Registered claim for Track B (the graph-defect fixes)

Written before the diff lands: **this changes no retrieval metric.** The prior probe measured the
corroboration fix at today's `0.15` weight to *regress* the hard corpus (recall@1 98 → 97, MRR
0.9900 → 0.9850) and to be a no-op at `w ≤ 0.05` (**PROVISIONAL-UNREPRODUCED**). It ships as a
**correctness** fix that makes the arm live and measurable, never as an improvement. Neo4j is not
started for this experiment and Track B gates nothing.

**Evidence-preservation rule (Gap 28):** `eval/graph_attach_probe_test.go` is currently **untracked**
(**MEASURED-HERE**, `git status`). It must be **committed first**, with a header saying it is
measurement scaffolding, and only then deleted in the Track B commit — so git retains the record of
what it measured. Deleting an untracked file destroys its own evidence.

---

## 6. WHAT THIS CANNOT PROVE

1. **Not abstraction, and therefore not the "semantic knowledge" half of the thesis.** `Synthesize`
   and `ExtractTriples` return `ErrNoLLMCredential`; `configs/config.go:110` expands
   `${OPENAI_API_KEY}` to `""` and `main.go:113` builds an OpenAI provider that 401s per cluster.
   **Consolidation as shipped is a no-op that reports success:** `Synthesize` fails first
   (`worker.go:124`), its handler is a per-cluster `continue` (`worker.go:126-128`), `consolidatedIDs`
   stays nil, `MarkConsolidated` never runs, and `ProcessTask` returns `nil` — asynq records success
   — while `ConsolidationRuns.Inc()` and `ClustersFormed` have **already** fired in the deferred block
   at `worker.go:82-85`/`:110`. No consolidated unit in this experiment asserts anything no member
   asserted.
2. **Not the repo's designed supersession mechanism.** `conflict.go` in its entirety has never
   executed: its only caller `worker.go:141` sits behind `ExtractTriples`. D1/D2 are episode-level
   reinventions of it and must be described as such (Gap 1).
3. **Not answer quality.** There is no judge. LongMemEval's official scorer requires
   `OPENAI_API_KEY` with gpt-4o; LoCoMo's free-text answers need one too. This measures whether
   forgetting puts the **right evidence** in front of a reader, not whether a better answer comes
   out. `llm.Generate` has zero production callers — there is no generation step to evaluate.
4. **Not the production read path (Gap 2).** `workspace.go:26` is
   `retrieval → DIG → knapsack → context assembly`; production emits a **packed context**, not a
   ranked list. This experiment measures the ranked list only. `hard_eval_test.go:62`'s existing
   caveat — never imply knapsack assembly was measured — is carried forward verbatim. Note also that
   `force_recent_turns: 3` (`configs/config.yaml:42`, **MEASURED-HERE**) is **already a recency prior
   in production**, so D3 is not a novel rival: it is a knob production already ships.
5. **Not the production write path (Gap 3).** Episodes are built directly, bypassing
   `internal/segmentation` and `internal/ingest`, because `models.NewEpisode` hardcodes
   `time.Now()` (`models.go:52-67`) and `ingest.Service` never overrides `Timestamp`. Consequence,
   stated: **the recently-fixed ingest path is not exercised here, so "ingest is fixed" gains no
   evidence from this experiment.**
6. **Not the production consolidation worker end to end (Gap 4 — the plan's §4.7 was false about its
   own contents).** **No arm calls `Worker.ProcessTask`.** D0 calls
   `consolidation.NewDBSCAN(0.3,3).Cluster` directly; `ProcessTask` cannot get past `Synthesize`
   without a credential. The scheduler cannot be driven offline at all: `scheduler.go:86` discovers
   users from an in-process `sync.Map` fed only by `main.go:223/247`, so a user is invisible until
   they hit the API in this process lifetime and a restart forgets everyone.
   `GetUnconsolidated`'s hard-coded limit of 100 (`worker.go:96`) also caps a real cycle below the
   corpus size.
7. **ORACLE is a ceiling, not a result.** `SR@1 = 1.0` there is arithmetic. ANTI-ORACLE exists
   because ORACLE alone would pass identically with the gold columns swapped.
8. **SR@1 alone is gameable, and is structurally blind to collateral damage.** FLOOR reaches 1.0 by
   destroying memory. Per §3.1, in the primary row SR@1 cannot even *see* a non-gold archive. Only
   the joint rule with the 398-query control makes any SR@1 number meaningful; **SR@1 must never be
   quoted without control recall@5 beside it.**
9. **Not compounding decay.** `n` is capped at 1 — `MarkConsolidated` (`worker.go:161`) precedes any
   possible re-selection and `GetUnconsolidated` filters `status=pending` (`qdrant.go:211-232`) — so
   `r^n` has no dynamic range in this codebase. `UpdateDecay` is a flat `SetPayload` of a constant
   (`qdrant.go:283-305`): it overwrites, never compounds. Compounding needs a repeat-visit mechanism
   that does not exist.
10. **Not generality.** One dataset, one embedder, one language, English assistant-chat, ~24-turn
    memories, one clusterer, one detector family. All 142 gold turns in the primary set carry
    `role == "user"` (**MEASURED-HERE**) — the metric never tests supersession of assistant content.
11. **A truncated index.** 44.9 % of turns are cut at 256 wordpieces
    (**PROVISIONAL-UNREPRODUCED**). Distractors are weaker than the raw corpus size suggests.
12. **The escape hatch deliberately not taken.** ~4.9 GB of VRAM and 18 GB of disk are free; a 1–2 GB
    quantized instruct model would fit and would unlock real `Synthesize`/`ExtractTriples` and an
    answer judge. It is not taken here because it introduces prompt and decoding as new unfrozen
    knobs and requires the same model and prompt in both arms. If the decay result is positive, that
    is the next experiment, not this one.
13. **Supersedes, does not confirm.** `cma/paper.tex:67` and its table at `:675-687` assert LoCoMo
    gains of +66.7 % temporal and +28.4 % multi-hop that **no code in this repo produced**. The
    write-up must state in one sentence that this measurement **supersedes** that table, or a modest
    real delta will be read as validating a fabricated one.

---

## 7. GAP REGISTER — all 29, each resolved or explicitly scoped out

| Gap | Resolution | § |
|---|---|---|
| 1 | `conflict.go` is the designed supersession mechanism, unreachable behind `ExtractTriples`; D1/D2 named as episode-level reinventions of it, not novel | 4.5, 6.2 |
| 2 | knapsack/workspace out of scope, stated; `force_recent_turns: 3` named as an existing production recency prior, so D3 is not novel | 6.4 |
| 3 | segmentation/ingest bypassed; consequence stated — "ingest is fixed" gains no evidence here | 6.5 |
| 4 | corrected: **no arm calls `Worker.ProcessTask`**; D0 calls `NewDBSCAN(...).Cluster` directly | 6.6 |
| 5 | datasets copied to `~/data/memora-eval/` on the real fs before this commit; URLs, shas, `SHA256SUMS`, and `LICENSE-LoCoMo.txt` recorded; sha verification at harness startup (V8) | 4.1 |
| 6 | archived-count via raw Qdrant HTTP `points/count`; collection drop via raw `DELETE`; no new interface method | 4.12 |
| 7 | harness must use `sharedMetrics()` (`shared_test.go:50`); a fresh `metrics.New()` panics the package binary | 4.9 |
| 8 | RAM re-measured today: 15 GB total / **5 available**, 8 GB swap in use, `/` at 90 % with 18 GB free | 4.1 |
| 9 | `Wait` latency measured before/after and reported; **rollback rule** at > 2× regression | 4.15 |
| 10 | `cmd/probe` named; **every PROVISIONAL number is struck from the record if it cannot be reproduced** | 4.10 |
| 11 | reconciliation given teeth as **V4**, the RAW SR@1 premise gate `[0.35, 0.65]` | 5.2 |
| 12 | degenerate assert replaced by **PF0–PF4**; the `DecayFactor = 0.0` trap becomes its own runnable check | 4.11 |
| 13 | pooled row gets its own collection, own detector pass, own archive application; order fixed as upsert → detect → archive → search | 4.13 |
| 14 | collection `cma_eval_forget`, dim **384**, cosine; `vector_size: 1536` not read; `cma_episodes` never touched | 4.9 |
| 15 | IDF corpus **per-instance** (pooled only for the pooled row), with reasons and the thin-corpus limitation stated | 4.7 |
| 16 | nearest-older fully pinned: total order, tie-break, same-session allowed, **one pass / no cascade**, no role filter | 4.4, 4.5 |
| 17 | "10 %" replaced by **ρ\* = 0.0439**, the ORACLE arm's own archive rate (72/1640, MEASURED); selection on the 407 control instances; **no grid extension**; ties to the lower rate | 4.6 |
| 18 | the **9 `_abs`** items excluded from the falsifier (n = 398) and reported as a labelled exploratory row; KU-permissive contains 0 | 4.3 |
| 19 | **one** primary hypothesis, **one** primary cell, **one** confirmatory test; everything else labelled EXPLORATORY with the test count printed | 1.1, 1.2, 4.14 |
| 20 | pooled row and LoCoMo control each get a named metric, a named threshold, and an explicit EXPLORATORY (no-p-value) status | 4.13 |
| 21 | rival clause rewritten as `> max(D3 best, RAW + 10 pp)`; vacuous-satisfaction wording registered | 2.3 |
| 22 | **RANDOM-ARCHIVE**: per-instance exact rate match, matched eligibility set, 20 seeded replicates, D2 must beat the max | 2.1 |
| 23 | **ANTI-ORACLE**: `SR@1 = 0.0` exactly, plus a defined-query vacuity guard | 2.2 |
| 24 | mandatory 2×2 per-query attribution table, with the §3.1 arithmetic that makes the right-hand column a bug check | 3.2 |
| 25 | **V3** (evidence-found `< 0.95`) and **V4** (RAW SR@1 outside `[0.35, 0.65]`) — the premise-broken thresholds, named now | 5.2 |
| 26 | `ArchiveByIDs` on `*QdrantStore`, **not** on the interface — one implementation, no mocks, eval holds the concrete type | 4.12 |
| 27 | scoped out with the invariant recorded: `Upsert` always writes `consolidation_status` and the eval collection is created fresh, so the missing-key case cannot arise; no test built | 4.11 |
| 28 | `graph_attach_probe_test.go` must be **committed before it is deleted**, so git holds the record | 5.3 |
| 29 | LoCoMo CC BY-NC 4.0 licence file on disk with its sha; attribution + stripped-subset-sha publication gate before any LoCoMo number | 4.1 |

---

## 8. AMENDMENTS

*(none — any amendment is appended here, dated, with its reason; nothing above is deleted)*
