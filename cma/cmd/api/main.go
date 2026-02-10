package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
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
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))
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
