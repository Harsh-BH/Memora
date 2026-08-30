package eval

import (
	"sync"

	"github.com/memora/cma/internal/metrics"
)

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
