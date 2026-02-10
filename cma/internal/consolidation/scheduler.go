package consolidation

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"

	"github.com/memora/cma/configs"
	"github.com/memora/cma/internal/vectorstore"
)

// Scheduler periodically checks for users that need consolidation
// and enqueues Asynq tasks. This implements the CMA "Sleep trigger"
// that fires on inactivity or unconsolidated episode threshold.
type Scheduler struct {
	worker       *Worker
	vectorDB     vectorstore.VectorStore
	asynqClient  *asynq.Client
	redisClient  *redis.Client
	cfg          configs.ConsolidationConfig
	lastActivity sync.Map // map[userID]time.Time
	stopCh       chan struct{}
}

// NewScheduler creates a new consolidation scheduler.
func NewScheduler(
	worker *Worker,
	vectorDB vectorstore.VectorStore,
	asynqClient *asynq.Client,
	redisClient *redis.Client,
	cfg configs.ConsolidationConfig,
) *Scheduler {
	return &Scheduler{
		worker:      worker,
		vectorDB:    vectorDB,
		asynqClient: asynqClient,
		redisClient: redisClient,
		cfg:         cfg,
		stopCh:      make(chan struct{}),
	}
}

// Start begins the periodic consolidation check loop.
func (s *Scheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(s.cfg.CheckInterval)
	defer ticker.Stop()

	slog.Info("consolidation scheduler started",
		"check_interval", s.cfg.CheckInterval,
		"inactivity_timeout", s.cfg.InactivityTimeout,
		"max_unconsolidated", s.cfg.MaxUnconsolidated,
	)

	for {
		select {
		case <-ticker.C:
			s.checkAllUsers(ctx)
		case <-s.stopCh:
			slog.Info("consolidation scheduler stopped")
			return
		case <-ctx.Done():
			slog.Info("consolidation scheduler context cancelled")
			return
		}
	}
}

// Stop signals the scheduler to stop.
func (s *Scheduler) Stop() {
	close(s.stopCh)
}

// RecordActivity updates the last activity timestamp for a user.
// Called by the ingest and query paths to track user wakefulness.
func (s *Scheduler) RecordActivity(userID string) {
	s.lastActivity.Store(userID, time.Now().UTC())
}

// checkAllUsers iterates over known users and enqueues consolidation
// tasks for those meeting the trigger criteria.
func (s *Scheduler) checkAllUsers(ctx context.Context) {
	s.lastActivity.Range(func(key, value any) bool {
		userID, ok := key.(string)
		if !ok {
			return true
		}
		lastAct, ok := value.(time.Time)
		if !ok {
			return true
		}

		shouldConsolidate, reason := s.worker.ShouldConsolidate(ctx, userID, lastAct)
		if !shouldConsolidate {
			return true
		}

		// Try to acquire a Redis lock for this user's consolidation.
		// This prevents concurrent consolidation of the same user.
		lockKey := "cma:consolidation:lock:" + userID
		acquired, err := s.redisClient.SetNX(ctx, lockKey, "locked", 5*time.Minute).Result()
		if err != nil {
			slog.Error("redis lock failed", "user_id", userID, "error", err)
			return true
		}
		if !acquired {
			slog.Debug("consolidation already running", "user_id", userID)
			return true
		}

		// Enqueue consolidation task.
		task, err := NewConsolidateTask(userID)
		if err != nil {
			slog.Error("create task failed", "user_id", userID, "error", err)
			return true
		}

		info, err := s.asynqClient.Enqueue(task)
		if err != nil {
			slog.Error("enqueue consolidation failed", "user_id", userID, "error", err)
			// Release the lock on failure.
			s.redisClient.Del(ctx, lockKey)
			return true
		}

		slog.Info("consolidation enqueued",
			"user_id", userID,
			"reason", reason,
			"task_id", info.ID,
		)

		return true
	})
}
