package eval

import (
	"sync"

	"github.com/memora/cma/internal/metrics"
)

// HARNESS BUG, found 2026-08-31 while running the hybrid eval: every test in
// this package creates a uniquely-timestamped Qdrant collection
// (cma_hardeval_*, cma_rrfeval_*, cma_hybrideval_*) and NONE of them ever
// deletes it. Collections therefore accumulate in the shared cma_qdrant
// container, one per test per run, forever.
//
// That is not merely untidy. The container ships with a soft RLIMIT_NOFILE of
// 1024 (hard limit 524288), and each collection holds a set of RocksDB segment
// files open. Observed: after roughly three accumulated eval collections
// Qdrant begins logging "Too many open files (os error 24)", stops accepting
// connections on both 6333 and 6334, and every test here fails at
// EnsureCollection with "failed to receive server preface within timeout" --
// which looks like a network problem and is not one.
//
// WORKAROUND used during the round-6 measurements, from the host:
//
//	docker restart cma_qdrant
//	curl -s -X DELETE localhost:6333/collections/<each stale cma_*eval_* name>
//
// (Restart, not recreate -- the named volume cma_qdrant_data and the
// production cma_episodes collection must survive.)
//
// TWO REAL FIXES, neither applied here:
//   - Harness side: drop the collection in a t.Cleanup. vectorstore.VectorStore
//     exposes no DeleteCollection, so this needs an interface addition or a
//     raw HTTP call from the test -- a deliberate choice, not a one-liner.
//   - Container side: run cma_qdrant with --ulimit nofile=65536:524288 on the
//     same named volume. This is the actual root cause; 1024 is simply too low
//     for Qdrant. Recreating the container was explicitly out of scope for the
//     round these tests were written in.

// metrics.New() registers every metric on prometheus's global default
// registry via promauto, so calling it more than once in the same test
// binary panics with "duplicate metrics collector registration attempted."
// Both eval tests in this package need a *metrics.Metrics; share one
// instance rather than constructing a second.
var (
	sharedMetricsOnce sync.Once
	sharedMetricsVal  *metrics.Metrics
)

func sharedMetrics() *metrics.Metrics {
	sharedMetricsOnce.Do(func() {
		sharedMetricsVal = metrics.New()
	})
	return sharedMetricsVal
}
