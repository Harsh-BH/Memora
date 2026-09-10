package eval

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/memora/cma/configs"
	"github.com/memora/cma/internal/models"
	"github.com/memora/cma/internal/vectorstore"
)

// TestArchiveByIDsRemovesFromSearch is the runnable check on the forgetting
// mechanism itself: before this change nothing in the repo could archive
// anything (models.StatusArchived had zero references, DeleteByIDs had zero
// callers, and Search had no status filter), so "archiving hides a point from
// retrieval" was an untested assertion.
//
// It also pins the two properties the eval harness depends on and that are
// easy to break silently:
//   - CountByStatus reflects the archive immediately (WaitForIndex: true).
//   - A point with consolidation_status "pending" is still returned, i.e. the
//     MustNot clause excludes ONLY archived points, not everything.
//
// Deliberately synthetic 8-dim vectors: this tests the payload filter, not the
// embedding model, and loading ONNX for it would make the check slower and
// skippable for an unrelated reason.
func TestArchiveByIDsRemovesFromSearch(t *testing.T) {
	if !qdrantReachable("localhost:6334") {
		t.Skipf("Qdrant not reachable at localhost:6334 -- see README for how to start it")
	}
	ctx := context.Background()

	const dim = 8
	collectionName := fmt.Sprintf("cma_archivetest_%d", time.Now().UnixNano())
	store, err := vectorstore.NewQdrantStore(configs.QdrantConfig{
		Host: "localhost", GRPCPort: 6334,
		Collection: collectionName, VectorSize: dim, HnswM: 16, HnswEF: 100,
		WaitForIndex: true,
	})
	if err != nil {
		t.Fatalf("NewQdrantStore: %v", err)
	}
	defer store.Close()
	if err := store.EnsureCollection(ctx); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}
	t.Cleanup(func() { dropCollection(t, collectionName) })

	const userID = "archive-test-user"
	// Three points on the same axis so their cosine order is fixed and known:
	// ep0 is the nearest to the query, ep2 the farthest.
	eps := make([]models.Episode, 3)
	for i := range eps {
		vec := make([]float32, dim)
		vec[0] = 1.0
		vec[1] = float32(i) * 0.5 // larger i => farther from the query
		eps[i] = models.Episode{
			ID:                  fmt.Sprintf("00000000-0000-4000-8000-00000000000%d", i),
			UserID:              userID,
			Content:             fmt.Sprintf("episode %d", i),
			Embedding:           vec,
			Timestamp:           time.Unix(1700000000+int64(i), 0),
			MemoryType:          models.MemoryEpisodic,
			ConsolidationStatus: models.StatusPending,
			DecayFactor:         1.0,
		}
	}
	if err := store.Upsert(ctx, eps); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	query := make([]float32, dim)
	query[0] = 1.0

	ids := func(t *testing.T) []string {
		t.Helper()
		res, err := store.Search(ctx, userID, query, 10)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		out := make([]string, 0, len(res))
		for _, r := range res {
			out = append(out, r.Episode.ID)
		}
		return out
	}

	got := ids(t)
	if len(got) != 3 {
		t.Fatalf("before archiving: Search returned %d points, want 3 (%v)", len(got), got)
	}
	if got[0] != eps[0].ID {
		t.Fatalf("before archiving: nearest point is %s, want %s -- the fixture's cosine order is wrong, "+
			"so the post-archive assertion below would not mean anything", got[0], eps[0].ID)
	}

	if n, err := store.CountByStatus(ctx, userID, models.StatusArchived); err != nil || n != 0 {
		t.Fatalf("CountByStatus(archived) before archiving = %d, %v; want 0, nil", n, err)
	}

	if err := store.ArchiveByIDs(ctx, []string{eps[0].ID}); err != nil {
		t.Fatalf("ArchiveByIDs: %v", err)
	}

	got = ids(t)
	if len(got) != 2 {
		t.Fatalf("after archiving 1 of 3: Search returned %d points, want 2 (%v)", len(got), got)
	}
	for _, id := range got {
		if id == eps[0].ID {
			t.Fatalf("after archiving: Search still returned the archived point %s -- "+
				"Search's MustNot consolidation_status filter is not working", id)
		}
	}
	if got[0] != eps[1].ID {
		t.Fatalf("after archiving the nearest point, top-1 is %s, want the runner-up %s", got[0], eps[1].ID)
	}

	if n, err := store.CountByStatus(ctx, userID, models.StatusArchived); err != nil || n != 1 {
		t.Fatalf("CountByStatus(archived) after archiving 1 = %d, %v; want 1, nil", n, err)
	}
	if n, err := store.CountByStatus(ctx, userID, models.StatusPending); err != nil || n != 2 {
		t.Fatalf("CountByStatus(pending) after archiving 1 of 3 = %d, %v; want 2, nil -- "+
			"the MustNot filter must exclude archived points only", n, err)
	}

	// Empty id list must be a no-op, not an error: the eval harness calls
	// ArchiveByIDs for every arm, and RAW's archive set is empty by definition.
	if err := store.ArchiveByIDs(ctx, nil); err != nil {
		t.Fatalf("ArchiveByIDs(nil) = %v, want nil (RAW's archive set is empty)", err)
	}
}
