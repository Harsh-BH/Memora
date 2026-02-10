package graphstore

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"

	"github.com/memora/cma/configs"
	"github.com/memora/cma/internal/models"
)

// Neo4jStore implements GraphStore using the Neo4j Go driver.
type Neo4jStore struct {
	driver   neo4j.DriverWithContext
	database string
}

// NewNeo4jStore creates a new Neo4j-backed GraphStore.
func NewNeo4jStore(cfg configs.Neo4jConfig) (*Neo4jStore, error) {
	driver, err := neo4j.NewDriverWithContext(
		cfg.URI,
		neo4j.BasicAuth(cfg.Username, cfg.Password, ""),
	)
	if err != nil {
		return nil, fmt.Errorf("neo4j driver: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := driver.VerifyConnectivity(ctx); err != nil {
		return nil, fmt.Errorf("neo4j connectivity: %w", err)
	}

	return &Neo4jStore{
		driver:   driver,
		database: cfg.Database,
	}, nil
}

// EnsureSchema creates constraints and indexes for the CMA graph schema.
func (n *Neo4jStore) EnsureSchema(ctx context.Context) error {
	session := n.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: n.database, AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	constraints := []string{
		"CREATE CONSTRAINT entity_id IF NOT EXISTS FOR (e:Entity) REQUIRE e.id IS UNIQUE",
		"CREATE CONSTRAINT concept_id IF NOT EXISTS FOR (c:Concept) REQUIRE c.id IS UNIQUE",
		"CREATE CONSTRAINT event_id IF NOT EXISTS FOR (ev:Event) REQUIRE ev.id IS UNIQUE",
		"CREATE INDEX entity_name IF NOT EXISTS FOR (e:Entity) ON (e.name)",
		"CREATE INDEX entity_user IF NOT EXISTS FOR (e:Entity) ON (e.user_id)",
		"CREATE INDEX concept_user IF NOT EXISTS FOR (c:Concept) ON (c.user_id)",
	}

	for _, cypher := range constraints {
		_, err := session.Run(ctx, cypher, nil)
		if err != nil {
			slog.Warn("neo4j schema", "cypher", cypher, "error", err)
		}
	}

	slog.Info("neo4j schema ensured")
	return nil
}

// InsertTriple creates a new semantic triple with bi-temporal metadata.
// This is ONLY called during the consolidation (Sleep) cycle.
func (n *Neo4jStore) InsertTriple(ctx context.Context, userID string, triple models.Triple, sourceEpID string) error {
	session := n.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: n.database, AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	now := time.Now().UTC()

	cypher := `
		MERGE (s:Entity {name: $subject, user_id: $user_id})
		ON CREATE SET s.id = $subject_id, s.created_at = datetime($now), s.last_accessed = datetime($now)
		ON MATCH SET s.last_accessed = datetime($now)

		MERGE (o:Entity {name: $object, user_id: $user_id})
		ON CREATE SET o.id = $object_id, o.created_at = datetime($now), o.last_accessed = datetime($now)
		ON MATCH SET o.last_accessed = datetime($now)

		CREATE (s)-[r:RELATES_TO {
			id: $rel_id,
			predicate: $predicate,
			confidence: $confidence,
			valid_from: datetime($now),
			transaction_time: datetime($now),
			source_ep_id: $source_ep_id,
			decay_rate: 1.0,
			user_id: $user_id
		}]->(o)

		RETURN r.id AS rel_id
	`

	params := map[string]any{
		"subject":      triple.Subject,
		"object":       triple.Object,
		"predicate":    triple.Predicate,
		"confidence":   triple.Confidence,
		"user_id":      userID,
		"subject_id":   uuid.New().String(),
		"object_id":    uuid.New().String(),
		"rel_id":       uuid.New().String(),
		"source_ep_id": sourceEpID,
		"now":          now.Format(time.RFC3339),
	}

	_, err := session.Run(ctx, cypher, params)
	if err != nil {
		return fmt.Errorf("neo4j insert triple: %w", err)
	}

	return nil
}

// QueryBySubject retrieves all relationships for a given subject entity filtered by user_id.
func (n *Neo4jStore) QueryBySubject(ctx context.Context, userID string, subject string) ([]models.GraphRelationship, error) {
	session := n.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: n.database, AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	cypher := `
		MATCH (s:Entity {name: $subject, user_id: $user_id})-[r:RELATES_TO]->(o:Entity)
		WHERE r.valid_to IS NULL OR r.valid_to > datetime()
		RETURN r.id AS id, s.name AS from_name, o.name AS to_name,
		       r.predicate AS predicate, r.confidence AS confidence,
		       r.valid_from AS valid_from, r.valid_to AS valid_to,
		       r.transaction_time AS transaction_time, r.source_ep_id AS source_ep_id,
		       r.decay_rate AS decay_rate
		ORDER BY r.confidence DESC
	`

	result, err := session.Run(ctx, cypher, map[string]any{
		"subject": subject,
		"user_id": userID,
	})
	if err != nil {
		return nil, fmt.Errorf("neo4j query by subject: %w", err)
	}

	var rels []models.GraphRelationship
	for result.Next(ctx) {
		record := result.Record()
		rel := recordToRelationship(record)
		rels = append(rels, rel)
	}

	return rels, result.Err()
}

// TraverseHops performs a variable-length path traversal up to maxHops from seed entities.
func (n *Neo4jStore) TraverseHops(ctx context.Context, userID string, seedEntities []string, maxHops int) ([]models.RetrievalResult, error) {
	session := n.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: n.database, AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	cypher := fmt.Sprintf(`
		MATCH path = (s:Entity {user_id: $user_id})-[r:RELATES_TO*1..%d]-(target:Entity)
		WHERE s.name IN $seeds
		  AND ALL(rel IN relationships(path) WHERE rel.valid_to IS NULL OR rel.valid_to > datetime())
		UNWIND relationships(path) AS rel
		WITH DISTINCT rel, startNode(rel) AS src, endNode(rel) AS dst
		RETURN rel.id AS id,
		       src.name AS from_name, dst.name AS to_name,
		       rel.predicate AS predicate, rel.confidence AS confidence,
		       rel.source_ep_id AS source_ep_id
		ORDER BY rel.confidence DESC
		LIMIT 50
	`, maxHops)

	result, err := session.Run(ctx, cypher, map[string]any{
		"user_id": userID,
		"seeds":   seedEntities,
	})
	if err != nil {
		return nil, fmt.Errorf("neo4j traverse: %w", err)
	}

	var results []models.RetrievalResult
	for result.Next(ctx) {
		record := result.Record()
		fromName, _ := record.Get("from_name")
		toName, _ := record.Get("to_name")
		predicate, _ := record.Get("predicate")
		confidence, _ := record.Get("confidence")

		factStr := fmt.Sprintf("%s %s %s", fromName, predicate, toName)
		conf := 0.0
		if c, ok := confidence.(float64); ok {
			conf = c
		}

		results = append(results, models.RetrievalResult{
			GraphFacts: []models.Triple{
				{
					Subject:    fmt.Sprintf("%v", fromName),
					Predicate:  fmt.Sprintf("%v", predicate),
					Object:     fmt.Sprintf("%v", toName),
					Confidence: conf,
				},
			},
			Score:  conf,
			Source: "graph",
			Episode: &models.Episode{
				Content: factStr,
			},
		})
	}

	return results, result.Err()
}

// FindConflicts checks if a new triple conflicts with existing facts in the graph.
func (n *Neo4jStore) FindConflicts(ctx context.Context, userID string, triple models.Triple) ([]models.ConflictRecord, error) {
	session := n.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: n.database, AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	// Find existing relationships with the same subject and predicate but different object.
	cypher := `
		MATCH (s:Entity {name: $subject, user_id: $user_id})-[r:RELATES_TO {predicate: $predicate}]->(o:Entity)
		WHERE o.name <> $object
		  AND (r.valid_to IS NULL OR r.valid_to > datetime())
		RETURN r.id AS rel_id, s.name AS subject, r.predicate AS predicate,
		       o.name AS object, r.confidence AS confidence
	`

	result, err := session.Run(ctx, cypher, map[string]any{
		"subject":   triple.Subject,
		"predicate": triple.Predicate,
		"object":    triple.Object,
		"user_id":   userID,
	})
	if err != nil {
		return nil, fmt.Errorf("neo4j find conflicts: %w", err)
	}

	var conflicts []models.ConflictRecord
	for result.Next(ctx) {
		record := result.Record()
		relID, _ := record.Get("rel_id")
		subj, _ := record.Get("subject")
		pred, _ := record.Get("predicate")
		obj, _ := record.Get("object")
		conf, _ := record.Get("confidence")

		confidence := 0.0
		if c, ok := conf.(float64); ok {
			confidence = c
		}

		conflicts = append(conflicts, models.ConflictRecord{
			ExistingRelID: fmt.Sprintf("%v", relID),
			ExistingTriple: models.Triple{
				Subject:    fmt.Sprintf("%v", subj),
				Predicate:  fmt.Sprintf("%v", pred),
				Object:     fmt.Sprintf("%v", obj),
				Confidence: confidence,
			},
			NewTriple:  triple,
			DetectedAt: time.Now().UTC(),
			Resolution: "",
		})
	}

	return conflicts, result.Err()
}

// ResolveConflict applies temporal decay to an old relationship by closing
// its valid_to window and reducing its confidence.
func (n *Neo4jStore) ResolveConflict(ctx context.Context, conflict models.ConflictRecord, decayRate float64) error {
	session := n.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: n.database, AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	now := time.Now().UTC()

	cypher := `
		MATCH ()-[r:RELATES_TO {id: $rel_id}]->()
		SET r.valid_to = datetime($now),
		    r.decay_rate = $decay_rate,
		    r.confidence = r.confidence * $decay_rate
		RETURN r.id AS id
	`

	_, err := session.Run(ctx, cypher, map[string]any{
		"rel_id":     conflict.ExistingRelID,
		"now":        now.Format(time.RFC3339),
		"decay_rate": decayRate,
	})
	if err != nil {
		return fmt.Errorf("neo4j resolve conflict: %w", err)
	}

	return nil
}

// Close releases the Neo4j driver.
func (n *Neo4jStore) Close(ctx context.Context) error {
	return n.driver.Close(ctx)
}

// --- Helpers ---

func recordToRelationship(record *neo4j.Record) models.GraphRelationship {
	rel := models.GraphRelationship{}

	if v, ok := record.Get("id"); ok {
		rel.ID = fmt.Sprintf("%v", v)
	}
	if v, ok := record.Get("predicate"); ok {
		rel.RelationType = fmt.Sprintf("%v", v)
	}
	if v, ok := record.Get("confidence"); ok {
		if c, ok := v.(float64); ok {
			rel.Confidence = c
		}
	}
	if v, ok := record.Get("source_ep_id"); ok {
		rel.SourceEpisodeID = fmt.Sprintf("%v", v)
	}
	if v, ok := record.Get("decay_rate"); ok {
		if d, ok := v.(float64); ok {
			rel.DecayRate = d
		}
	}
	if v, ok := record.Get("valid_from"); ok {
		if t, ok := v.(time.Time); ok {
			rel.ValidFrom = t
		}
	}
	if v, ok := record.Get("valid_to"); ok {
		if t, ok := v.(time.Time); ok {
			rel.ValidTo = &t
		}
	}
	if v, ok := record.Get("transaction_time"); ok {
		if t, ok := v.(time.Time); ok {
			rel.TransactionTime = t
		}
	}

	return rel
}
