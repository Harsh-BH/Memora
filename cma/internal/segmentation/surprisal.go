package segmentation

import (
	"context"
	"math"
	"strings"
	"sync"

	"github.com/memora/cma/configs"
	"github.com/memora/cma/internal/llm"
	"github.com/memora/cma/internal/models"
)

// SurprisalEngine implements Bayesian Surprise-based event segmentation.
//
// Mathematical basis (from the CMA paper):
//
//	Surprisal(x_t) = -log P(x_t | x_<t)
//
// An event boundary is triggered when:
//
//	S > μ + γσ
//
// where μ and σ are the rolling mean and standard deviation of recent
// surprisal values over a window τ.
type SurprisalEngine struct {
	llmProvider llm.Provider
	gamma       float64 // sensitivity parameter, typically ∈ [1, 2]
	windowSize  int     // rolling stats window τ
	minTokens   int     // minimum tokens per episode
	maxTokens   int     // maximum tokens per episode

	// Per-user rolling statistics for adaptive thresholding.
	mu    sync.Map // map[userID]*rollingStats
}

// rollingStats maintains a sliding window of surprisal values for
// computing dynamic thresholds per user.
type rollingStats struct {
	values []float64
	mu     float64
	sigma  float64
	idx    int
	full   bool
	size   int
	lock   sync.Mutex
}

func newRollingStats(windowSize int) *rollingStats {
	return &rollingStats{
		values: make([]float64, windowSize),
		size:   windowSize,
	}
}

// update adds a new surprisal value and recomputes rolling μ and σ.
func (r *rollingStats) update(s float64) {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.values[r.idx] = s
	r.idx = (r.idx + 1) % r.size
	if r.idx == 0 {
		r.full = true
	}

	n := r.size
	if !r.full {
		n = r.idx
	}
	if n == 0 {
		return
	}

	// Compute mean.
	sum := 0.0
	for i := 0; i < n; i++ {
		sum += r.values[i]
	}
	r.mu = sum / float64(n)

	// Compute standard deviation.
	varSum := 0.0
	for i := 0; i < n; i++ {
		diff := r.values[i] - r.mu
		varSum += diff * diff
	}
	r.sigma = math.Sqrt(varSum / float64(n))
}

// threshold returns the current boundary threshold: μ + γσ.
func (r *rollingStats) threshold(gamma float64) float64 {
	r.lock.Lock()
	defer r.lock.Unlock()
	return r.mu + gamma*r.sigma
}

func (r *rollingStats) mean() float64 {
	r.lock.Lock()
	defer r.lock.Unlock()
	return r.mu
}

// NewSurprisalEngine creates a new event segmentation engine.
func NewSurprisalEngine(provider llm.Provider, cfg configs.SegmentationConfig) *SurprisalEngine {
	return &SurprisalEngine{
		llmProvider: provider,
		gamma:       cfg.Gamma,
		windowSize:  cfg.WindowSize,
		minTokens:   cfg.MinEpisodeTokens,
		maxTokens:   cfg.MaxEpisodeTokens,
	}
}

// Segment processes raw text input and segments it into episodic fragments
// based on surprisal-driven event boundaries.
func (s *SurprisalEngine) Segment(ctx context.Context, userID string, text string) ([]models.Episode, error) {
	// Get or create per-user rolling stats.
	statsIface, _ := s.mu.LoadOrStore(userID, newRollingStats(s.windowSize))
	stats := statsIface.(*rollingStats)

	// Get token-level log probabilities from the LLM.
	tokenProbs, err := s.llmProvider.GetTokenProbabilities(ctx, text)
	if err != nil {
		// Fallback: treat entire text as single episode.
		return s.singleEpisode(ctx, userID, text)
	}

	if len(tokenProbs) == 0 {
		return s.singleEpisode(ctx, userID, text)
	}

	// Compute surprisal for each token and detect boundaries.
	var episodes []models.Episode
	var currentTokens []string
	var maxSurprisal float64
	var totalSurprisal float64
	tokenCount := 0

	for _, tp := range tokenProbs {
		// Surprisal(x_t) = -log P(x_t | x_<t)
		surprisal := -tp.LogProb
		if surprisal < 0 {
			surprisal = 0
		}

		// Update rolling statistics.
		stats.update(surprisal)

		// Check boundary condition: S > μ + γσ
		threshold := stats.threshold(s.gamma)
		isBoundary := surprisal > threshold && tokenCount >= s.minTokens

		// Also force boundary at max token limit.
		if tokenCount >= s.maxTokens {
			isBoundary = true
		}

		if isBoundary && len(currentTokens) > 0 {
			// Emit episode.
			content := strings.Join(currentTokens, " ")
			avgSurprisal := totalSurprisal / float64(tokenCount)
			ep, err := s.createEpisode(ctx, userID, content, math.Max(avgSurprisal, maxSurprisal))
			if err != nil {
				return nil, err
			}
			ep.TokenCount = tokenCount
			episodes = append(episodes, *ep)

			// Reset accumulator.
			currentTokens = nil
			maxSurprisal = 0
			totalSurprisal = 0
			tokenCount = 0
		}

		currentTokens = append(currentTokens, tp.Token)
		totalSurprisal += surprisal
		if surprisal > maxSurprisal {
			maxSurprisal = surprisal
		}
		tokenCount++
	}

	// Emit final episode if there are remaining tokens.
	if len(currentTokens) > 0 {
		content := strings.Join(currentTokens, " ")
		avgSurprisal := 0.0
		if tokenCount > 0 {
			avgSurprisal = totalSurprisal / float64(tokenCount)
		}
		ep, err := s.createEpisode(ctx, userID, content, math.Max(avgSurprisal, maxSurprisal))
		if err != nil {
			return nil, err
		}
		ep.TokenCount = tokenCount
		episodes = append(episodes, *ep)
	}

	return episodes, nil
}

// createEpisode builds an Episode with embedding.
func (s *SurprisalEngine) createEpisode(ctx context.Context, userID string, content string, surprisal float64) (*models.Episode, error) {
	embedding, err := s.llmProvider.Embed(ctx, content)
	if err != nil {
		return nil, err
	}

	ep := models.NewEpisode(userID, content, embedding, surprisal)
	ep.TokenCount = s.llmProvider.CountTokens(content)
	return ep, nil
}

// singleEpisode treats the entire input as one episode (fallback).
func (s *SurprisalEngine) singleEpisode(ctx context.Context, userID string, text string) ([]models.Episode, error) {
	ep, err := s.createEpisode(ctx, userID, text, 1.0)
	if err != nil {
		return nil, err
	}
	return []models.Episode{*ep}, nil
}
