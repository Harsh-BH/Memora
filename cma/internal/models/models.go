package models

import (
	"time"

	"github.com/google/uuid"
)

// --- Enums ---

// ConsolidationStatus represents the lifecycle state of an episodic fragment.
type ConsolidationStatus string

const (
	StatusPending      ConsolidationStatus = "pending"
	StatusConsolidated ConsolidationStatus = "consolidated"
	StatusArchived     ConsolidationStatus = "archived"
)

// MemoryType distinguishes episodic from semantic memory.
type MemoryType string

const (
	MemoryEpisodic MemoryType = "episodic"
	MemorySemantic MemoryType = "semantic"
)

// --- Core Domain Types ---

// Episode represents a segmented episodic memory fragment produced by the
// surprisal-based segmentation engine. This is the hippocampal unit of storage.
type Episode struct {
	ID                  string              `json:"id"`
	UserID              string              `json:"user_id"`
	Content             string              `json:"content"`
	Embedding           []float32           `json:"embedding"`
	Timestamp           time.Time           `json:"timestamp"`
	EventID             string              `json:"event_id"`
	MemoryType          MemoryType          `json:"memory_type"`
	ImportanceScore     float64             `json:"importance_score"`
	ConsolidationStatus ConsolidationStatus `json:"consolidation_status"`
	SurprisalValue      float64             `json:"surprisal_value"`
	AssociatedEntities  []string            `json:"associated_entities"`
	DecayFactor         float64             `json:"decay_factor"`
	TokenCount          int                 `json:"token_count"`
	Metadata            map[string]any      `json:"metadata,omitempty"`
}

// NewEpisode creates a new Episode with sensible defaults.
func NewEpisode(userID, content string, embedding []float32, surprisal float64) *Episode {
	now := time.Now().UTC()
	return &Episode{
		ID:                  uuid.New().String(),
		UserID:              userID,
		Content:             content,
		Embedding:           embedding,
		Timestamp:           now,
		EventID:             uuid.New().String(),
		MemoryType:          MemoryEpisodic,
		ImportanceScore:     surprisal, // initial importance = surprisal
		ConsolidationStatus: StatusPending,
		SurprisalValue:      surprisal,
		AssociatedEntities:  []string{},
		DecayFactor:         1.0,
		Metadata:            make(map[string]any),
	}
}

// --- Knowledge Graph Types ---

// Triple represents a semantic (Subject, Predicate, Object) fact extracted
// by the consolidation engine. This is the neocortical unit of knowledge.
type Triple struct {
	Subject    string  `json:"subject"`
	Predicate  string  `json:"predicate"`
	Object     string  `json:"object"`
	Confidence float64 `json:"confidence"`
}

// GraphEntity represents a node in the Neo4j knowledge graph.
type GraphEntity struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	EntityType   string    `json:"type"`
	Embedding    []float32 `json:"embedding,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	LastAccessed time.Time `json:"last_accessed"`
	Properties   map[string]any `json:"properties,omitempty"`
}

// GraphRelationship represents an edge in the Neo4j knowledge graph
// with bi-temporal modeling (valid_time + transaction_time).
type GraphRelationship struct {
	ID              string    `json:"id"`
	FromEntityID    string    `json:"from_entity_id"`
	ToEntityID      string    `json:"to_entity_id"`
	RelationType    string    `json:"relation_type"`
	Confidence      float64   `json:"confidence"`
	ValidFrom       time.Time `json:"valid_from"`
	ValidTo         *time.Time `json:"valid_to,omitempty"` // nil = currently valid
	TransactionTime time.Time `json:"transaction_time"`
	SourceEpisodeID string    `json:"source_ep_id"`
	DecayRate       float64   `json:"decay_rate"`
	Properties      map[string]any `json:"properties,omitempty"`
}

// --- Consolidation Types ---

// ConsolidationJob represents a unit of work for the sleep-cycle worker.
type ConsolidationJob struct {
	UserID     string     `json:"user_id"`
	TimeWindow TimeRange  `json:"time_window"`
	Episodes   []Episode  `json:"episodes"`
}

// TimeRange specifies a half-open time interval [Start, End).
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// Cluster represents a group of semantically related episodes
// identified by DBSCAN during consolidation.
type Cluster struct {
	ID        int       `json:"id"`
	Episodes  []Episode `json:"episodes"`
	Centroid  []float32 `json:"centroid"`
}

// ConflictRecord captures a detected contradiction between existing
// graph facts and a newly extracted proposition.
type ConflictRecord struct {
	ExistingRelID string    `json:"existing_rel_id"`
	ExistingTriple Triple   `json:"existing_triple"`
	NewTriple      Triple   `json:"new_triple"`
	DetectedAt     time.Time `json:"detected_at"`
	Resolution     string    `json:"resolution"` // "update", "discard", "coexist"
}

// --- Retrieval Types ---

// RetrievalResult wraps a memory fragment with its retrieval metadata.
type RetrievalResult struct {
	Episode      *Episode   `json:"episode,omitempty"`
	GraphFacts   []Triple   `json:"graph_facts,omitempty"`
	Score        float64    `json:"score"`
	Source       string     `json:"source"` // "vector" or "graph"
}

// DIGCandidate is a retrieval result annotated with its Document
// Information Gain score for reranking.
type DIGCandidate struct {
	Result   RetrievalResult `json:"result"`
	DIGScore float64         `json:"dig_score"`
	Content  string          `json:"content"`
}

// KnapsackItem represents a candidate memory fragment for context
// window packing via Lagrangian relaxation.
type KnapsackItem struct {
	ID         string  `json:"id"`
	Content    string  `json:"content"`
	Value      float64 `json:"value"`       // DIG score or importance
	Weight     int     `json:"weight"`      // Token count
	ForceInclude bool  `json:"force_include"` // For recent turns
	Density    float64 `json:"density"`     // value / weight
}

// --- Workspace Types ---

// WorkspaceContext is the assembled working memory buffer sent to the LLM
// for final response generation.
type WorkspaceContext struct {
	UserID         string          `json:"user_id"`
	Query          string          `json:"query"`
	RecentTurns    []ConversationTurn `json:"recent_turns"`
	SelectedItems  []KnapsackItem  `json:"selected_items"`
	TotalTokens    int             `json:"total_tokens"`
	TokenBudget    int             `json:"token_budget"`
}

// ConversationTurn holds a single user/assistant exchange.
type ConversationTurn struct {
	Role      string    `json:"role"` // "user" or "assistant"
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// --- API Request/Response Types ---

// IngestRequest is the API payload for ingesting new memory.
type IngestRequest struct {
	UserID  string `json:"user_id" binding:"required"`
	Content string `json:"content" binding:"required"`
	Role    string `json:"role" binding:"required"` // "user" or "assistant"
}

// IngestResponse returns metadata about the ingested episodes.
type IngestResponse struct {
	EpisodeIDs []string `json:"episode_ids"`
	Segments   int      `json:"segments"`
	Message    string   `json:"message"`
}

// QueryRequest is the API payload for querying memory.
type QueryRequest struct {
	UserID     string `json:"user_id" binding:"required"`
	Query      string `json:"query" binding:"required"`
	TokenBudget int   `json:"token_budget,omitempty"`
}

// QueryResponse returns the assembled context and metadata.
type QueryResponse struct {
	Context       string            `json:"context"`
	Sources       []RetrievalResult `json:"sources"`
	TokensUsed    int               `json:"tokens_used"`
	TokenBudget   int               `json:"token_budget"`
	DIGScores     map[string]float64 `json:"dig_scores,omitempty"`
}

// HealthResponse is returned by the health check endpoint.
type HealthResponse struct {
	Status    string            `json:"status"`
	Version   string            `json:"version"`
	Services  map[string]string `json:"services"`
	Timestamp time.Time         `json:"timestamp"`
}
