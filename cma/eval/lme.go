package eval

// LongMemEval / LoCoMo loading for the forgetting experiment.
//
// Everything here is a rule from cma/eval/PREREGISTRATION.md turned into code,
// with the section number on each. Nothing in this file may be changed to make
// a number come out differently -- the counts it produces were published before
// any arm ran, and the tests in forget_test.go pin them.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// DatasetDir is the durable, non-tmpfs home of the corpora (PREREGISTRATION.md
// 4.1). /tmp on the development box is tmpfs, so a reboot deletes anything
// registered there; the experiment reads from the real filesystem only.
// Overridable by MEMORA_EVAL_DATA for a machine that keeps them elsewhere --
// the sha256 check below is what actually pins identity, not the path.
func DatasetDir() string {
	if d := os.Getenv("MEMORA_EVAL_DATA"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join("/home", "harsh1", "data", "memora-eval")
	}
	return filepath.Join(home, "data", "memora-eval")
}

// Registered sha256 digests (PREREGISTRATION.md 4.1, testdata/DATASETS.md).
// A mismatch is void condition V8: the corpus is not the registered corpus.
const (
	SHA256LongMemEval = "821a2034d219ab45846873dd14c14f12cfe7776e73527a483f9dac095d38620c"
	SHA256LoCoMo      = "79fa87e90f04081343b8c8debecb80a9a6842b76a7aa537dc9fdf651ea698ff4"
)

// readVerified reads a file and fails if its sha256 is not the registered one.
func readVerified(path, wantSHA string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w (see cma/eval/testdata/DATASETS.md for the "+
			"canonical location and source URL)", path, err)
	}
	sum := sha256.Sum256(raw)
	if got := hex.EncodeToString(sum[:]); got != wantSHA {
		return nil, fmt.Errorf("VOID (V8): %s sha256 = %s, registered = %s -- "+
			"the corpus is not the registered corpus", path, got, wantSHA)
	}
	return raw, nil
}

// --- LongMemEval ---

// LMETurn is one conversational turn. has_answer is the corpus's own gold
// evidence flag; content and role are verbatim.
type LMETurn struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	HasAnswer bool   `json:"has_answer"`
}

// LMEInstance is one LongMemEval question with its haystack.
type LMEInstance struct {
	QuestionID   string `json:"question_id"`
	QuestionType string `json:"question_type"`
	Question     string `json:"question"`
	// Answer is raw JSON, not string: 32 of the 500 answers are JSON NUMBERS
	// rather than strings (MEASURED 2026-09-01, e.g. question_id 0a995998 has
	// answer 3). Nothing in this experiment reads it -- there is no judge and no
	// generation step (PREREGISTRATION.md 6.3) -- so it is kept verbatim rather
	// than coerced into a type the corpus does not have.
	Answer             json.RawMessage `json:"answer"`
	QuestionDate       string          `json:"question_date"`
	HaystackDates      []string        `json:"haystack_dates"`
	HaystackSessionIDs []string        `json:"haystack_session_ids"`
	HaystackSessions   [][]LMETurn     `json:"haystack_sessions"`
	AnswerSessionIDs   []string        `json:"answer_session_ids"`
}

// IsAbstention reports whether this is one of the 30 "_abs" items whose task is
// to abstain, so retrieving the flagged turn is not the goal (4.3). Nine of
// them sit inside the 407-query control set and are excluded from the falsifier.
func (in *LMEInstance) IsAbstention() bool { return strings.HasSuffix(in.QuestionID, "_abs") }

// lmeDateLayout parses "2023/04/10 (Mon) 23:07". All 948 haystack_dates and all
// 500 question_dates in the registered file match it (MEASURED 2026-09-01).
const lmeDateLayout = "2006/01/02 (Mon) 15:04"

// Turn is one turn flattened out of an instance, carrying its identity, its
// 4.4 ordering position, and the deterministic point ID it will hold in Qdrant.
type Turn struct {
	Instance    *LMEInstance
	SessionIdx  int       // index into HaystackSessions
	TurnIdx     int       // position within that session
	SessionDate time.Time // parsed HaystackDates[SessionIdx]
	Role        string
	Content     string
	HasAnswer   bool
	ID          string // deterministic UUIDv5, stable across runs and processes
}

// turnNamespace makes turn IDs reproducible: the same corpus always yields the
// same point IDs, in this process and in cmd/probe, so a frozen gold map keyed
// by UUID stays valid. uuid.New() would not survive a re-run.
var turnNamespace = uuid.NewSHA1(uuid.NameSpaceOID, []byte("memora/eval/forgetting/turn"))

func turnID(questionID string, sessionIdx, turnIdx int) string {
	return uuid.NewSHA1(turnNamespace, fmt.Appendf(nil, "%s#%d#%d", questionID, sessionIdx, turnIdx)).String()
}

// Timestamp is the episode timestamp written for the record and for D3's
// delta-days: session date plus turn_idx minutes (4.4). It is deliberately NOT
// the ordering authority -- Less is.
func (t Turn) Timestamp() time.Time {
	return t.SessionDate.Add(time.Duration(t.TurnIdx) * time.Minute)
}

// Less is the strict total order of PREREGISTRATION.md 4.4:
//
//	key(turn) = ( parse(haystack_dates[i]), i, turn_idx )
//
// lexicographic ascending. Every "older"/"newer" statement in the experiment
// resolves against this and never against a float timestamp, so a timestamp
// collision cannot introduce ambiguity. The key is strict: turn_idx is unique
// within a session and the session index disambiguates same-dated sessions.
func (t Turn) Less(o Turn) bool {
	if !t.SessionDate.Equal(o.SessionDate) {
		return t.SessionDate.Before(o.SessionDate)
	}
	if t.SessionIdx != o.SessionIdx {
		return t.SessionIdx < o.SessionIdx
	}
	return t.TurnIdx < o.TurnIdx
}

// Turns flattens the instance into 4.4 order. Ingest uses this order because
// clustering.go:82-84 flips a border point's noise label on first touch, so
// unsorted ingestion makes the D0 arm unreproducible.
func (in *LMEInstance) Turns() ([]Turn, error) {
	var out []Turn
	for si, sess := range in.HaystackSessions {
		if si >= len(in.HaystackDates) {
			return nil, fmt.Errorf("%s: session %d has no haystack_date", in.QuestionID, si)
		}
		date, err := time.Parse(lmeDateLayout, in.HaystackDates[si])
		if err != nil {
			return nil, fmt.Errorf("%s: haystack_dates[%d] = %q: %w", in.QuestionID, si, in.HaystackDates[si], err)
		}
		for ti, turn := range sess {
			out = append(out, Turn{
				Instance: in, SessionIdx: si, TurnIdx: ti, SessionDate: date,
				Role: turn.Role, Content: turn.Content, HasAnswer: turn.HasAnswer,
				ID: turnID(in.QuestionID, si, ti),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Less(out[j]) })
	return out, nil
}

// QuestionTime parses question_date, used by D3 for its recency prior.
func (in *LMEInstance) QuestionTime() (time.Time, error) {
	return time.Parse(lmeDateLayout, in.QuestionDate)
}

// LoadLongMemEval reads and sha-verifies the registered corpus.
func LoadLongMemEval(path string) ([]*LMEInstance, error) {
	raw, err := readVerified(path, SHA256LongMemEval)
	if err != nil {
		return nil, err
	}
	var out []*LMEInstance
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return out, nil
}

// LongMemEvalPath is the registered file's canonical location.
func LongMemEvalPath() string { return filepath.Join(DatasetDir(), "lme_oracle.json") }

// --- Query sets (PREREGISTRATION.md 4.2), stated as rules, not ID lists ---

// IsKUPermissive is the PRIMARY set's rule, and the only rule that selects the
// confirmatory cell. Two clauses, no free parameter:
//
//	question_type == "knowledge-update", and for each of the 2
//	answer_session_ids the corresponding haystack session contains at least
//	one turn with has_answer == true.
//
// MEASURED 2026-09-01: n = 70, of which 0 are abstention items. The plan's
// alternative rule ("exactly 2 has_answer flags") yields 68 and is NOT this.
func IsKUPermissive(in *LMEInstance) bool {
	if in.QuestionType != "knowledge-update" || len(in.AnswerSessionIDs) != 2 {
		return false
	}
	for _, want := range in.AnswerSessionIDs {
		idx := -1
		for i, sid := range in.HaystackSessionIDs {
			if sid == want {
				idx = i
				break
			}
		}
		if idx < 0 || idx >= len(in.HaystackSessions) {
			return false
		}
		found := false
		for _, t := range in.HaystackSessions[idx] {
			if t.HasAnswer {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// IsControl is the control set's rule: any non-knowledge-update instance with
// at least one gold turn anywhere. MEASURED: n = 407, of which 9 are
// abstention items.
func IsControl(in *LMEInstance) bool {
	if in.QuestionType == "knowledge-update" {
		return false
	}
	for _, sess := range in.HaystackSessions {
		for _, t := range sess {
			if t.HasAnswer {
				return true
			}
		}
	}
	return false
}

// IsControlPrimary is the FALSIFIER set: control minus the 9 abstention items
// (4.3). Those 9 are 2.2% of the control against a 2pp falsifier threshold, so
// including them could flip the falsifier by itself. MEASURED: n = 398.
func IsControlPrimary(in *LMEInstance) bool { return IsControl(in) && !in.IsAbstention() }

// Select filters instances by predicate, preserving file order.
func Select(all []*LMEInstance, pred func(*LMEInstance) bool) []*LMEInstance {
	var out []*LMEInstance
	for _, in := range all {
		if pred(in) {
			out = append(out, in)
		}
	}
	return out
}

// --- The gold map (PREREGISTRATION.md 3, 4.6) ---

// Gold is one instance's frozen supersession labels, by turn ID.
//
// Stale is the gold turns in the OLDER gold-bearing session, Current those in
// the NEWEST gold-bearing session, "older"/"newer" under the 4.4 order. Getting
// this backwards is the failure the ANTI-ORACLE arm exists to catch: ORACLE
// reaches SR@1 = 1.0 identically with the two columns swapped, so it proves
// nothing about orientation on its own.
type Gold struct {
	Stale   map[string]bool
	Current map[string]bool
}

// IsGold reports whether a turn ID carries either label.
func (g Gold) IsGold(id string) bool { return g.Stale[id] || g.Current[id] }

// GoldOf builds the supersession labels for one instance from its turns.
//
// MEASURED 2026-09-01 over KU-permissive: 72 stale and 70 current gold turns
// (two instances carry two stale golds) out of 1640 turns, which is the
// rho* = 0.0439 target archive rate published in 4.6 before any arm ran. All
// 142 are role == "user".
func GoldOf(turns []Turn) Gold {
	g := Gold{Stale: map[string]bool{}, Current: map[string]bool{}}
	// turns must be in 4.4 order (Turns returns it that way), in which sessions
	// are contiguous blocks: the LAST gold turn is therefore in the newest
	// gold-bearing session, and one pass finds it.
	newestGoldSession := -1
	for _, t := range turns {
		if t.HasAnswer {
			newestGoldSession = t.SessionIdx
		}
	}
	for _, t := range turns {
		if !t.HasAnswer {
			continue
		}
		if t.SessionIdx == newestGoldSession {
			g.Current[t.ID] = true
		} else {
			g.Stale[t.ID] = true
		}
	}
	return g
}

// GoldIDs returns the gold turn IDs of an instance regardless of stale/current,
// which is what the control set's recall@5 needs (it has no supersession).
func GoldIDs(turns []Turn) map[string]bool {
	out := map[string]bool{}
	for _, t := range turns {
		if t.HasAnswer {
			out[t.ID] = true
		}
	}
	return out
}

// --- LoCoMo ---

// LoCoMoSample is one LoCoMo conversation with its questions. The GPT-written
// gist fields are deliberately absent from this struct: see LoadLoCoMo.
type LoCoMoSample struct {
	SampleID     string         `json:"sample_id"`
	QA           []LoCoMoQA     `json:"qa"`
	Conversation map[string]any `json:"conversation"`
}

// LoCoMoQA is one question. Evidence holds dia_id strings such as "D1:3".
type LoCoMoQA struct {
	Question string   `json:"question"`
	Answer   any      `json:"answer"`
	Evidence []string `json:"evidence"`
	Category int      `json:"category"`
}

// loCoMoBannedFields are GPT-written gists tagged with the SAME dia_ids the
// gold evidence points to. Reading any of them -- to build units, to seed
// clustering, to define grouping, or even to pick a threshold -- is a
// guaranteed, meaningless win. PREREGISTRATION.md 4.1 requires the strip be
// enforced in code, not prose, which is what LoadLoCoMo does and what
// TestLoCoMoStripsGistFields checks.
var loCoMoBannedFields = []string{"observation", "session_summary", "event_summary"}

// LoadLoCoMo reads and sha-verifies LoCoMo, and strips the gist fields before
// returning anything. It parses into a generic map first precisely so the strip
// happens on the raw object: decoding straight into LoCoMoSample would drop the
// fields from the struct while leaving them in the bytes, which is not the same
// guarantee.
//
// LoCoMo is CC BY-NC 4.0 (see cma/eval/testdata/DATASETS.md). No LoCoMo number
// may be published until the licence file is on disk, the attribution line is
// in the write-up, use is research-only, and the sha256 of the stripped subset
// is published first.
func LoadLoCoMo(path string) ([]*LoCoMoSample, error) {
	raw, err := readVerified(path, SHA256LoCoMo)
	if err != nil {
		return nil, err
	}
	var generic []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	out := make([]*LoCoMoSample, 0, len(generic))
	for i, obj := range generic {
		for _, banned := range loCoMoBannedFields {
			delete(obj, banned)
		}
		stripped, err := json.Marshal(obj)
		if err != nil {
			return nil, fmt.Errorf("re-marshal sample %d: %w", i, err)
		}
		var s LoCoMoSample
		if err := json.Unmarshal(stripped, &s); err != nil {
			return nil, fmt.Errorf("parse sample %d: %w", i, err)
		}
		out = append(out, &s)
	}
	return out, nil
}

// LoCoMoPath is the registered file's canonical location.
func LoCoMoPath() string { return filepath.Join(DatasetDir(), "locomo10.json") }
