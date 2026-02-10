package workspace

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/memora/cma/configs"
	"github.com/memora/cma/internal/dig"
	"github.com/memora/cma/internal/knapsack"
	"github.com/memora/cma/internal/metrics"
	"github.com/memora/cma/internal/models"
	"github.com/memora/cma/internal/retrieval"
)

// Workspace implements the Cognitive Workspace (Layer 2 in the CMA paper).
// It acts as the working memory buffer, implementing Baddeley's
// Multi-Component Model:
//
//   - Phonological Loop: retains last N turns of raw dialogue
//   - Episodic Buffer: retrieved long-term memories integrated with current context
//   - Active Management via Knapsack optimizer for token budget enforcement
//
// Read path: retrieval → DIG reranking → knapsack optimization → context assembly.
type Workspace struct {
	retriever  *retrieval.Service
	reranker   *dig.Reranker
	optimizer  *knapsack.Optimizer
	cfg        configs.KnapsackConfig
	metrics    *metrics.Metrics

	// Per-user conversation history (phonological loop).
	// In production, this would be backed by Redis.
	history    map[string][]models.ConversationTurn
}

// NewWorkspace creates a new cognitive workspace.
func NewWorkspace(
	retriever *retrieval.Service,
	reranker *dig.Reranker,
	optimizer *knapsack.Optimizer,
	cfg configs.KnapsackConfig,
	m *metrics.Metrics,
) *Workspace {
	return &Workspace{
		retriever: retriever,
		reranker:  reranker,
		optimizer: optimizer,
		cfg:       cfg,
		metrics:   m,
		history:   make(map[string][]models.ConversationTurn),
	}
}

// Query executes the full read path and returns assembled context.
func (w *Workspace) Query(ctx context.Context, req models.QueryRequest) (*models.QueryResponse, error) {
	start := time.Now()

	// Override token budget if specified in request.
	tokenBudget := w.cfg.TokenBudget
	if req.TokenBudget > 0 {
		tokenBudget = req.TokenBudget
	}

	slog.Info("workspace query",
		"user_id", req.UserID,
		"query", req.Query,
		"token_budget", tokenBudget,
	)

	// Step 1: Hybrid retrieval (concurrent vector + graph search).
	results, err := w.retriever.Retrieve(ctx, req.UserID, req.Query)
	if err != nil {
		return nil, fmt.Errorf("retrieval: %w", err)
	}

	// Step 2: DIG reranking — filter distractors, rank by information gain.
	digCandidates, err := w.reranker.Rerank(ctx, req.Query, results)
	if err != nil {
		return nil, fmt.Errorf("dig reranking: %w", err)
	}

	// Record DIG metrics.
	filteredCount := len(results) - len(digCandidates)
	w.metrics.DIGFilteredCount.Add(float64(filteredCount))
	for _, dc := range digCandidates {
		w.metrics.DIGScoreHist.Observe(dc.DIGScore)
	}

	// Step 3: Convert DIG candidates to knapsack items.
	knapsackItems := make([]models.KnapsackItem, 0, len(digCandidates))
	digScores := make(map[string]float64)

	for _, dc := range digCandidates {
		tokenCount := len(dc.Content) / 4 // approximate
		if tokenCount == 0 {
			tokenCount = 1
		}

		id := ""
		if dc.Result.Episode != nil {
			id = dc.Result.Episode.ID
		}

		item := models.KnapsackItem{
			ID:      id,
			Content: dc.Content,
			Value:   dc.DIGScore,
			Weight:  tokenCount,
		}

		knapsackItems = append(knapsackItems, item)
		if id != "" {
			digScores[id] = dc.DIGScore
		}
	}

	// Step 4: Knapsack optimization — pack context window with highest-value items.
	recentTurns := w.getRecentTurns(req.UserID)

	// Temporarily override the optimizer budget.
	w.optimizer.SetTokenBudget(tokenBudget)
	selection := w.optimizer.Optimize(knapsackItems, recentTurns)

	// Record knapsack metrics.
	w.metrics.KnapsackUtilization.Observe(selection.Utilization)
	w.metrics.KnapsackItemsSelected.Observe(float64(len(selection.Selected)))

	// Step 5: Assemble context string.
	contextStr := w.assembleContext(selection.Selected, req.Query)

	// Build sources list.
	sources := make([]models.RetrievalResult, 0, len(digCandidates))
	for _, dc := range digCandidates {
		sources = append(sources, dc.Result)
	}

	slog.Info("workspace query completed",
		"user_id", req.UserID,
		"candidates", len(results),
		"after_dig", len(digCandidates),
		"selected", len(selection.Selected),
		"tokens_used", selection.TotalTokens,
		"utilization", selection.Utilization,
		"latency_ms", time.Since(start).Milliseconds(),
	)

	return &models.QueryResponse{
		Context:     contextStr,
		Sources:     sources,
		TokensUsed:  selection.TotalTokens,
		TokenBudget: tokenBudget,
		DIGScores:   digScores,
	}, nil
}

// AddTurn appends a conversation turn to the user's phonological loop.
func (w *Workspace) AddTurn(userID string, role string, content string) {
	turn := models.ConversationTurn{
		Role:      role,
		Content:   content,
		Timestamp: time.Now().UTC(),
	}
	w.history[userID] = append(w.history[userID], turn)

	// Keep only the last 100 turns to bound memory.
	if len(w.history[userID]) > 100 {
		w.history[userID] = w.history[userID][len(w.history[userID])-100:]
	}
}

// getRecentTurns returns the conversation history for a user.
func (w *Workspace) getRecentTurns(userID string) []models.ConversationTurn {
	turns, ok := w.history[userID]
	if !ok {
		return nil
	}
	return turns
}

// assembleContext builds the final context string from selected knapsack items.
func (w *Workspace) assembleContext(items []models.KnapsackItem, query string) string {
	var sb strings.Builder

	// Recent conversation turns (force-included items).
	turnItems := make([]models.KnapsackItem, 0)
	memoryItems := make([]models.KnapsackItem, 0)

	for _, item := range items {
		if item.ForceInclude {
			turnItems = append(turnItems, item)
		} else {
			memoryItems = append(memoryItems, item)
		}
	}

	if len(turnItems) > 0 {
		sb.WriteString("## Recent Conversation\n")
		for _, item := range turnItems {
			sb.WriteString(item.Content)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	if len(memoryItems) > 0 {
		sb.WriteString("## Retrieved Memories\n")
		for i, item := range memoryItems {
			sb.WriteString(fmt.Sprintf("[Memory %d] %s\n", i+1, item.Content))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Current Query\n")
	sb.WriteString(query)

	return sb.String()
}
