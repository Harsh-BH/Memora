package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus metric instruments for the CMA system.
type Metrics struct {
	// Ingest
	EpisodesIngested  prometheus.Counter
	IngestLatency     prometheus.Histogram
	SegmentsBoundary  prometheus.Counter

	// Retrieval
	RetrievalLatency  prometheus.Histogram
	VectorSearchCount prometheus.Counter
	GraphSearchCount  prometheus.Counter

	// DIG
	DIGScoreHist      prometheus.Histogram
	DIGFilteredCount  prometheus.Counter

	// Knapsack
	KnapsackUtilization prometheus.Histogram
	KnapsackItemsSelected prometheus.Histogram

	// Consolidation
	ConsolidationRuns    prometheus.Counter
	ConsolidationLatency prometheus.Histogram
	ClustersFormed       prometheus.Histogram
	TriplesExtracted     prometheus.Counter
	ConflictsDetected    prometheus.Counter
	ConflictsResolved    prometheus.Counter
	EpisodesConsolidated prometheus.Counter

	// HTTP
	HTTPRequestsTotal   *prometheus.CounterVec
	HTTPRequestDuration *prometheus.HistogramVec
}

// New creates and registers all Prometheus metrics.
func New() *Metrics {
	return &Metrics{
		// --- Ingest ---
		EpisodesIngested: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "cma",
			Subsystem: "ingest",
			Name:      "episodes_total",
			Help:      "Total number of episodes ingested.",
		}),
		IngestLatency: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: "cma",
			Subsystem: "ingest",
			Name:      "latency_seconds",
			Help:      "Ingest pipeline latency in seconds.",
			Buckets:   prometheus.DefBuckets,
		}),
		SegmentsBoundary: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "cma",
			Subsystem: "ingest",
			Name:      "boundaries_detected_total",
			Help:      "Total number of event boundaries detected by surprisal engine.",
		}),

		// --- Retrieval ---
		RetrievalLatency: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: "cma",
			Subsystem: "retrieval",
			Name:      "latency_seconds",
			Help:      "Hybrid retrieval latency in seconds.",
			Buckets:   prometheus.DefBuckets,
		}),
		VectorSearchCount: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "cma",
			Subsystem: "retrieval",
			Name:      "vector_searches_total",
			Help:      "Total vector (Qdrant) searches.",
		}),
		GraphSearchCount: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "cma",
			Subsystem: "retrieval",
			Name:      "graph_searches_total",
			Help:      "Total graph (Neo4j) searches.",
		}),

		// --- DIG ---
		DIGScoreHist: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: "cma",
			Subsystem: "dig",
			Name:      "score_distribution",
			Help:      "Distribution of DIG scores.",
			Buckets:   prometheus.LinearBuckets(-1.0, 0.25, 12),
		}),
		DIGFilteredCount: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "cma",
			Subsystem: "dig",
			Name:      "filtered_total",
			Help:      "Total candidates filtered by DIG <= 0.",
		}),

		// --- Knapsack ---
		KnapsackUtilization: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: "cma",
			Subsystem: "knapsack",
			Name:      "utilization_ratio",
			Help:      "Ratio of token budget used by knapsack optimizer.",
			Buckets:   prometheus.LinearBuckets(0, 0.1, 11),
		}),
		KnapsackItemsSelected: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: "cma",
			Subsystem: "knapsack",
			Name:      "items_selected",
			Help:      "Number of items selected by knapsack optimizer.",
			Buckets:   prometheus.LinearBuckets(0, 5, 10),
		}),

		// --- Consolidation ---
		ConsolidationRuns: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "cma",
			Subsystem: "consolidation",
			Name:      "runs_total",
			Help:      "Total consolidation runs.",
		}),
		ConsolidationLatency: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: "cma",
			Subsystem: "consolidation",
			Name:      "latency_seconds",
			Help:      "Consolidation worker latency in seconds.",
			Buckets:   []float64{0.5, 1, 2, 5, 10, 30, 60, 120},
		}),
		ClustersFormed: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: "cma",
			Subsystem: "consolidation",
			Name:      "clusters_formed",
			Help:      "Number of clusters formed per consolidation run.",
			Buckets:   prometheus.LinearBuckets(0, 2, 10),
		}),
		TriplesExtracted: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "cma",
			Subsystem: "consolidation",
			Name:      "triples_extracted_total",
			Help:      "Total triples extracted by LLM.",
		}),
		ConflictsDetected: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "cma",
			Subsystem: "consolidation",
			Name:      "conflicts_detected_total",
			Help:      "Total conflicts detected during consolidation.",
		}),
		ConflictsResolved: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "cma",
			Subsystem: "consolidation",
			Name:      "conflicts_resolved_total",
			Help:      "Total conflicts resolved.",
		}),
		EpisodesConsolidated: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "cma",
			Subsystem: "consolidation",
			Name:      "episodes_consolidated_total",
			Help:      "Total episodes marked as consolidated.",
		}),

		// --- HTTP ---
		HTTPRequestsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "cma",
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total HTTP requests by method, path, and status.",
		}, []string{"method", "path", "status"}),
		HTTPRequestDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "cma",
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "HTTP request duration in seconds.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"method", "path"}),
	}
}
