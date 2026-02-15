package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"

	"github.com/memora/cma/configs"
	"github.com/memora/cma/internal/consolidation"
	"github.com/memora/cma/internal/dig"
	"github.com/memora/cma/internal/graphstore"
	"github.com/memora/cma/internal/ingest"
	"github.com/memora/cma/internal/knapsack"
	"github.com/memora/cma/internal/llm"
	"github.com/memora/cma/internal/metrics"
	"github.com/memora/cma/internal/middleware"
	"github.com/memora/cma/internal/models"
	"github.com/memora/cma/internal/retrieval"
	"github.com/memora/cma/internal/segmentation"
	"github.com/memora/cma/internal/vectorstore"
	"github.com/memora/cma/internal/workspace"
)

const version = "1.0.0"

func main() {
	// --- Configuration ---
	cfgPath := os.Getenv("CMA_CONFIG")
	if cfgPath == "" {
		cfgPath = "configs/config.yaml"
	}

	cfg, err := configs.Load(cfgPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// --- Logger ---
	logBuffer := NewLogBuffer(50)
	multiHandler := NewMultiHandler(
		slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}),
		logBuffer,
	)
	slog.SetDefault(slog.New(multiHandler))
	slog.Info("CMA starting", "version", version)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// --- Metrics ---
	m := metrics.New()

	// --- Infrastructure: Qdrant (Episodic Memory / Hippocampus) ---
	qdrantStore, err := vectorstore.NewQdrantStore(cfg.Qdrant)
	if err != nil {
		slog.Error("qdrant connection failed", "error", err)
		os.Exit(1)
	}
	defer qdrantStore.Close()

	if err := qdrantStore.EnsureCollection(ctx); err != nil {
		slog.Error("qdrant collection setup failed", "error", err)
		os.Exit(1)
	}

	// --- Infrastructure: Neo4j (Semantic Memory / Neocortex) ---
	neo4jStore, err := graphstore.NewNeo4jStore(cfg.Neo4j)
	if err != nil {
		slog.Error("neo4j connection failed", "error", err)
		os.Exit(1)
	}
	defer neo4jStore.Close(ctx)

	if err := neo4jStore.EnsureSchema(ctx); err != nil {
		slog.Error("neo4j schema setup failed", "error", err)
		os.Exit(1)
	}

	// --- Infrastructure: Redis ---
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer redisClient.Close()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		slog.Error("redis connection failed", "error", err)
		os.Exit(1)
	}

	// --- Infrastructure: Asynq ---
	asynqClient := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer asynqClient.Close()

	// --- LLM Provider ---
	llmProvider := llm.NewOpenAIProvider(cfg.LLM)

	// --- Domain Services ---

	// Segmentation engine (Bayesian Surprise).
	surprisalEngine := segmentation.NewSurprisalEngine(llmProvider, cfg.Segmentation)

	// Ingest pipeline (append-only episodic writes).
	ingestSvc := ingest.NewService(surprisalEngine, qdrantStore, m)

	// Retrieval service (concurrent vector + graph).
	retrievalSvc := retrieval.NewService(qdrantStore, neo4jStore, llmProvider, cfg.Retrieval, m)

	// DIG reranker.
	digReranker := dig.NewReranker(llmProvider, cfg.DIG)

	// Knapsack optimizer.
	knapsackOpt := knapsack.NewOptimizer(cfg.Knapsack)

	// Cognitive workspace (full read path).
	ws := workspace.NewWorkspace(retrievalSvc, digReranker, knapsackOpt, cfg.Knapsack, m)

	// Consolidation engine (Sleep cycle).
	dbscan := consolidation.NewDBSCAN(cfg.Consolidation.DBSCANEpsilon, cfg.Consolidation.DBSCANMinPoints)
	conflictResolver := consolidation.NewConflictResolver(neo4jStore, cfg.Consolidation.DecayRate)
	consolWorker := consolidation.NewWorker(qdrantStore, llmProvider, dbscan, conflictResolver, cfg.Consolidation, m)

	// Consolidation scheduler.
	consolScheduler := consolidation.NewScheduler(consolWorker, qdrantStore, asynqClient, redisClient, cfg.Consolidation)

	// --- Asynq Worker Server ---
	asynqSrv := asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     cfg.Redis.Addr,
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
		},
		asynq.Config{
			Concurrency: cfg.Consolidation.WorkerConcurrency,
			Queues: map[string]int{
				"consolidation": 10,
				"default":       5,
			},
		},
	)

	mux := asynq.NewServeMux()
	consolWorker.RegisterHandler(mux)

	go func() {
		if err := asynqSrv.Start(mux); err != nil {
			slog.Error("asynq server failed", "error", err)
		}
	}()

	// Start consolidation scheduler.
	go consolScheduler.Start(ctx)

	// --- HTTP Server (Gin) ---
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	// Middleware stack.
	router.Use(
		middleware.Recovery(),
		middleware.RequestID(),
		middleware.TenantExtractor(),
		middleware.CORS(),
		middleware.Logger(),
		middleware.PrometheusMiddleware(m),
	)

	// --- Routes ---

	// Health check.
	router.GET("/health", func(c *gin.Context) {
		services := map[string]string{
			"qdrant": "ok",
			"neo4j":  "ok",
			"redis":  "ok",
		}

		// Check Redis health.
		if err := redisClient.Ping(c.Request.Context()).Err(); err != nil {
			services["redis"] = "error: " + err.Error()
		}

		c.JSON(http.StatusOK, models.HealthResponse{
			Status:    "healthy",
			Version:   version,
			Services:  services,
			Timestamp: time.Now().UTC(),
		})
	})

	// Prometheus metrics.
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// API v1 group.
	v1 := router.Group("/api/v1")
	{
		// Ingest endpoint — append-only episodic writes.
		v1.POST("/ingest", func(c *gin.Context) {
			var req models.IngestRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			// Record activity for consolidation scheduler.
			consolScheduler.RecordActivity(req.UserID)

			// Add turn to workspace phonological loop.
			ws.AddTurn(req.UserID, req.Role, req.Content)

			resp, err := ingestSvc.Ingest(c.Request.Context(), req.UserID, req.Content, req.Role)
			if err != nil {
				slog.Error("ingest failed", "error", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "ingest failed"})
				return
			}

			c.JSON(http.StatusOK, resp)
		})

		// Query endpoint — cognitive workspace read path.
		v1.POST("/query", func(c *gin.Context) {
			var req models.QueryRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			// Record activity.
			consolScheduler.RecordActivity(req.UserID)

			resp, err := ws.Query(c.Request.Context(), req)
			if err != nil {
				slog.Error("query failed", "error", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
				return
			}

			c.JSON(http.StatusOK, resp)
		})

		// Hippocampus Stats Endpoint.
		v1.GET("/hippocampus", func(c *gin.Context) {
			userID := c.Query("user_id")
			if userID == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "user_id required"})
				return
			}

			episodes, err := qdrantStore.GetRecent(c.Request.Context(), userID, 50)
			if err != nil {
				slog.Error("hippocampus fetch failed", "error", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "fetch failed"})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"episodes": episodes,
				"stats":    gin.H{"total": len(episodes)}, // Placeholder for total count if expensive
			})
		})

		// Neocortex Stats Endpoint.
		v1.GET("/neocortex", func(c *gin.Context) {
			userID := c.Query("user_id")
			if userID == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "user_id required"})
				return
			}

			stats, err := neo4jStore.GetStats(c.Request.Context(), userID)
			if err != nil {
				slog.Error("neocortex stats failed", "error", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "fetch failed"})
				return
			}

			c.JSON(http.StatusOK, stats)
		})

		// System Logs Endpoint.
		v1.GET("/system/logs", func(c *gin.Context) {
			logs := logBuffer.GetLogs()
			c.JSON(http.StatusOK, logs)
		})

		// Workspace Context Endpoint.
		v1.GET("/workspace/context", func(c *gin.Context) {
			// In a real system, this would fetch current context based on user/session.
			// For now, we return a static/simulated context or fetch the last query.
			userID := c.Query("user_id")
			if userID == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "user_id required"})
				return
			}

			// TODO: Add strict "context" retrieval from Workspace service if available.
			// For now, let's return some stats about the workspace.
			
			// Mock data structure matching frontend expectations or new design
			c.JSON(http.StatusOK, gin.H{
				"id":           "ws_" + userID,
				"total_tokens": 2405, // TODO: Get from metrics or state
				"token_budget": 4096,
				"items": []gin.H{
					{"id": "kn_1", "content": "Query: Diff b/w Hippocampus & Neocortex", "weight": 12, "value": 0.99, "status": "kept"},
					{"id": "kn_2", "content": "[Graph] Hippocampus -> stores -> Episodes", "weight": 45, "value": 0.85, "status": "kept"},
					{"id": "kn_4", "content": "Prev Turn: User asked about architecture", "weight": 156, "value": 0.50, "status": "kept"},
				},
			})
		})

		// Admin: manually trigger consolidation.
		v1.POST("/admin/consolidate", func(c *gin.Context) {
			userID := c.Query("user_id")
			if userID == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "user_id required"})
				return
			}

			task, err := consolidation.NewConsolidateTask(userID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "create task failed"})
				return
			}

			info, err := asynqClient.Enqueue(task)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "enqueue failed"})
				return
			}

			c.JSON(http.StatusAccepted, gin.H{
				"message": "consolidation enqueued",
				"task_id": info.ID,
				"user_id": userID,
			})
		})
	}

	// --- Start HTTP Server ---
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	go func() {
		slog.Info("HTTP server starting", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server failed", "error", err)
			os.Exit(1)
		}
	}()

	// --- Graceful Shutdown ---
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down...")

	// Stop consolidation scheduler.
	consolScheduler.Stop()

	// Stop Asynq workers.
	asynqSrv.Shutdown()

	// Shutdown HTTP server with timeout.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("HTTP server shutdown error", "error", err)
	}

	cancel()
	slog.Info("CMA shutdown complete")
}

// --- Log Buffer Implementation ---

type LogEntry struct {
	Timestamp string `json:"ts"`
	Level     string `json:"level"`
	Module    string `json:"module"`
	Message   string `json:"msg"`
}

type LogBuffer struct {
	size    int
	buffer  []LogEntry
	mu      sync.Mutex
}

func NewLogBuffer(size int) *LogBuffer {
	return &LogBuffer{
		size:   size,
		buffer: make([]LogEntry, 0, size),
	}
}

func (lb *LogBuffer) Handle(ctx context.Context, r slog.Record) error {
	level := r.Level.String()
	msg := r.Message
	module := "System"

	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "module" {
			module = a.Value.String()
		}
		return true
	})

	entry := LogEntry{
		Timestamp: r.Time.Format("15:04:05"),
		Level:     level,
		Module:    module,
		Message:   msg,
	}

	lb.mu.Lock()
	defer lb.mu.Unlock()

	if len(lb.buffer) >= lb.size {
		lb.buffer = lb.buffer[1:]
	}
	lb.buffer = append(lb.buffer, entry)
	return nil
}

func (lb *LogBuffer) WithAttrs(attrs []slog.Attr) slog.Handler { return lb }
func (lb *LogBuffer) WithGroup(name string) slog.Handler   { return lb }
func (lb *LogBuffer) Enabled(ctx context.Context, level slog.Level) bool { return true }

func (lb *LogBuffer) GetLogs() []LogEntry {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	// Return a copy
	logs := make([]LogEntry, len(lb.buffer))
	copy(logs, lb.buffer)
	return logs
}

// --- MultiHandler ---

type MultiHandler struct {
	handlers []slog.Handler
}

func NewMultiHandler(handlers ...slog.Handler) *MultiHandler {
	return &MultiHandler{handlers: handlers}
}

func (m *MultiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m *MultiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, h := range m.handlers {
		_ = h.Handle(ctx, r.Clone())
	}
	return nil
}

func (m *MultiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		handlers[i] = h.WithAttrs(attrs)
	}
	return NewMultiHandler(handlers...)
}

func (m *MultiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		handlers[i] = h.WithGroup(name)
	}
	return NewMultiHandler(handlers...)
}

