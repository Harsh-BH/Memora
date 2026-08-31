package eval

import (
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/memora/cma/internal/metrics"
)

// qdrantHTTPAddr is Qdrant's REST port, as opposed to the 6334 gRPC port the
// vectorstore client uses. Only the raw-HTTP helpers here need it.
const qdrantHTTPAddr = "localhost:6333"

// dropCollection deletes a Qdrant collection over the REST API. Register it
// with t.Cleanup immediately after EnsureCollection succeeds; see the leak
// note above for what happens otherwise. Failures are logged, never fatal:
// a teardown that fails the test it is cleaning up after would hide the real
// result.
func dropCollection(t *testing.T, name string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete,
		fmt.Sprintf("http://%s/collections/%s", qdrantHTTPAddr, name), nil)
	if err != nil {
		t.Logf("CLEANUP: building DELETE for collection %q: %v", name, err)
		return
	}
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		t.Logf("CLEANUP: collection %q NOT dropped: %v -- drop it by hand "+
			"(curl -X DELETE %s/collections/%s) or Qdrant will hit its nofile limit",
			name, err, qdrantHTTPAddr, name)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Logf("CLEANUP: collection %q DELETE returned %s", name, resp.Status)
		return
	}
	t.Logf("CLEANUP: dropped Qdrant collection %q", name)
}

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
// FIXED, 2026-09-01, harness side: dropCollection below, called from a
// t.Cleanup in every test in this package that creates a collection. It is a
// raw HTTP DELETE rather than an interface method, because
// vectorstore.VectorStore exposes no DeleteCollection and adding one to the
// production interface purely for test teardown is not worth it (the same
// reasoning as ArchiveByIDs living on *QdrantStore -- see
// PREREGISTRATION.md 4.12).
//
// STILL NOT FIXED, container side: run cma_qdrant with
// --ulimit nofile=65536:524288 on the same named volume. That is the actual
// root cause -- 1024 is simply too low for Qdrant -- and docker cannot change
// a running container's ulimits, so applying it means recreating the
// container. Out of scope here; the named volume cma_qdrant_data and the
// production cma_episodes collection must survive any such attempt.

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
