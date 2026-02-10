package vectorstore

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	pb "github.com/qdrant/go-client/qdrant"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/memora/cma/configs"
	"github.com/memora/cma/internal/models"
)

// QdrantStore implements VectorStore using Qdrant's gRPC API.
type QdrantStore struct {
	conn       *grpc.ClientConn
	points     pb.PointsClient
	collections pb.CollectionsClient
	cfg        configs.QdrantConfig
}

// NewQdrantStore creates a new Qdrant-backed VectorStore.
func NewQdrantStore(cfg configs.QdrantConfig) (*QdrantStore, error) {
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.GRPCPort)
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("qdrant grpc dial: %w", err)
	}

	return &QdrantStore{
		conn:        conn,
		points:      pb.NewPointsClient(conn),
		collections: pb.NewCollectionsClient(conn),
		cfg:         cfg,
	}, nil
}

// EnsureCollection creates the cma_episodes collection with HNSW if not present.
func (q *QdrantStore) EnsureCollection(ctx context.Context) error {
	// Check if collection exists.
	listResp, err := q.collections.List(ctx, &pb.ListCollectionsRequest{})
	if err != nil {
		return fmt.Errorf("qdrant list collections: %w", err)
	}

	for _, col := range listResp.GetCollections() {
		if col.GetName() == q.cfg.Collection {
			slog.Info("qdrant collection already exists", "collection", q.cfg.Collection)
			return nil
		}
	}

	hnswM := q.cfg.HnswM
	if hnswM == 0 {
		hnswM = 16
	}
	hnswEF := q.cfg.HnswEF
	if hnswEF == 0 {
		hnswEF = 100
	}

	_, err = q.collections.Create(ctx, &pb.CreateCollection{
		CollectionName: q.cfg.Collection,
		VectorsConfig: &pb.VectorsConfig{
			Config: &pb.VectorsConfig_Params{
				Params: &pb.VectorParams{
					Size:     q.cfg.VectorSize,
					Distance: pb.Distance_Cosine,
					HnswConfig: &pb.HnswConfigDiff{
						M:            ptr(hnswM),
						EfConstruct:  ptr(hnswEF),
					},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("qdrant create collection: %w", err)
	}

	slog.Info("qdrant collection created", "collection", q.cfg.Collection)

	// Create payload indices for efficient filtering.
	payloadIndices := map[string]pb.FieldType{
		"user_id":              pb.FieldType_FieldTypeKeyword,
		"consolidation_status": pb.FieldType_FieldTypeKeyword,
		"memory_type":          pb.FieldType_FieldTypeKeyword,
		"timestamp":            pb.FieldType_FieldTypeInteger,
		"surprisal_value":      pb.FieldType_FieldTypeFloat,
		"decay_factor":         pb.FieldType_FieldTypeFloat,
	}

	for field, ftype := range payloadIndices {
		ft := ftype
		_, err := q.points.CreateFieldIndex(ctx, &pb.CreateFieldIndexCollection{
			CollectionName: q.cfg.Collection,
			FieldName:      field,
			FieldType:      &ft,
		})
		if err != nil {
			slog.Warn("qdrant create index", "field", field, "error", err)
		}
	}

	return nil
}

// Upsert stores episodic fragments as vectors with rich payloads.
func (q *QdrantStore) Upsert(ctx context.Context, episodes []models.Episode) error {
	points := make([]*pb.PointStruct, 0, len(episodes))

	for _, ep := range episodes {
		pointID := ep.ID
		if pointID == "" {
			pointID = uuid.New().String()
		}

		entities := make([]*pb.Value, 0, len(ep.AssociatedEntities))
		for _, e := range ep.AssociatedEntities {
			entities = append(entities, &pb.Value{
				Kind: &pb.Value_StringValue{StringValue: e},
			})
		}

		payload := map[string]*pb.Value{
			"content": {Kind: &pb.Value_StringValue{StringValue: ep.Content}},
			"event_id": {Kind: &pb.Value_StringValue{StringValue: ep.EventID}},
			"timestamp": {Kind: &pb.Value_IntegerValue{IntegerValue: ep.Timestamp.Unix()}},
			"user_id": {Kind: &pb.Value_StringValue{StringValue: ep.UserID}},
			"memory_type": {Kind: &pb.Value_StringValue{StringValue: string(ep.MemoryType)}},
			"importance_score": {Kind: &pb.Value_DoubleValue{DoubleValue: ep.ImportanceScore}},
			"consolidation_status": {Kind: &pb.Value_StringValue{StringValue: string(ep.ConsolidationStatus)}},
			"surprisal_value": {Kind: &pb.Value_DoubleValue{DoubleValue: ep.SurprisalValue}},
			"decay_factor": {Kind: &pb.Value_DoubleValue{DoubleValue: ep.DecayFactor}},
			"token_count": {Kind: &pb.Value_IntegerValue{IntegerValue: int64(ep.TokenCount)}},
			"associated_entities": {Kind: &pb.Value_ListValue{ListValue: &pb.ListValue{Values: entities}}},
		}

		points = append(points, &pb.PointStruct{
			Id:      &pb.PointId{PointIdOptions: &pb.PointId_Uuid{Uuid: pointID}},
			Vectors: &pb.Vectors{VectorsOptions: &pb.Vectors_Vector{Vector: &pb.Vector{Data: ep.Embedding}}},
			Payload: payload,
		})
	}

	_, err := q.points.Upsert(ctx, &pb.UpsertPoints{
		CollectionName: q.cfg.Collection,
		Points:         points,
	})
	if err != nil {
		return fmt.Errorf("qdrant upsert: %w", err)
	}

	return nil
}

// Search performs cosine similarity search with user_id payload filter.
func (q *QdrantStore) Search(ctx context.Context, userID string, queryVector []float32, topK int) ([]models.RetrievalResult, error) {
	resp, err := q.points.Search(ctx, &pb.SearchPoints{
		CollectionName: q.cfg.Collection,
		Vector:         queryVector,
		Limit:          uint64(topK),
		WithPayload:    &pb.WithPayloadSelector{SelectorOptions: &pb.WithPayloadSelector_Enable{Enable: true}},
		Filter: &pb.Filter{
			Must: []*pb.Condition{
				{
					ConditionOneOf: &pb.Condition_Field{
						Field: &pb.FieldCondition{
							Key: "user_id",
							Match: &pb.Match{
								MatchValue: &pb.Match_Keyword{Keyword: userID},
							},
						},
					},
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("qdrant search: %w", err)
	}

	results := make([]models.RetrievalResult, 0, len(resp.GetResult()))
	for _, hit := range resp.GetResult() {
		ep := payloadToEpisode(hit.GetId().GetUuid(), hit.GetPayload())
		results = append(results, models.RetrievalResult{
			Episode: ep,
			Score:   float64(hit.GetScore()),
			Source:  "vector",
		})
	}

	return results, nil
}

// GetUnconsolidated retrieves pending episodes for a user.
func (q *QdrantStore) GetUnconsolidated(ctx context.Context, userID string, limit int) ([]models.Episode, error) {
	resp, err := q.points.Scroll(ctx, &pb.ScrollPoints{
		CollectionName: q.cfg.Collection,
		Filter: &pb.Filter{
			Must: []*pb.Condition{
				{
					ConditionOneOf: &pb.Condition_Field{
						Field: &pb.FieldCondition{
							Key:   "user_id",
							Match: &pb.Match{MatchValue: &pb.Match_Keyword{Keyword: userID}},
						},
					},
				},
				{
					ConditionOneOf: &pb.Condition_Field{
						Field: &pb.FieldCondition{
							Key:   "consolidation_status",
							Match: &pb.Match{MatchValue: &pb.Match_Keyword{Keyword: string(models.StatusPending)}},
						},
					},
				},
			},
		},
		Limit:       ptr(uint32(limit)),
		WithPayload: &pb.WithPayloadSelector{SelectorOptions: &pb.WithPayloadSelector_Enable{Enable: true}},
		WithVectors: &pb.WithVectorsSelector{SelectorOptions: &pb.WithVectorsSelector_Enable{Enable: true}},
	})
	if err != nil {
		return nil, fmt.Errorf("qdrant scroll unconsolidated: %w", err)
	}

	episodes := make([]models.Episode, 0, len(resp.GetResult()))
	for _, pt := range resp.GetResult() {
		ep := payloadToEpisode(pt.GetId().GetUuid(), pt.GetPayload())
		if vec := pt.GetVectors().GetVector(); vec != nil {
			ep.Embedding = vec.GetData()
		}
		episodes = append(episodes, *ep)
	}

	return episodes, nil
}

// MarkConsolidated sets consolidation_status = "consolidated" for the given IDs.
func (q *QdrantStore) MarkConsolidated(ctx context.Context, ids []string) error {
	pointIDs := make([]*pb.PointId, 0, len(ids))
	for _, id := range ids {
		pointIDs = append(pointIDs, &pb.PointId{PointIdOptions: &pb.PointId_Uuid{Uuid: id}})
	}

	_, err := q.points.SetPayload(ctx, &pb.SetPayloadPoints{
		CollectionName: q.cfg.Collection,
		PointsSelector: &pb.PointsSelector{
			PointsSelectorOneOf: &pb.PointsSelector_Points{
				Points: &pb.PointsIdsList{Ids: pointIDs},
			},
		},
		Payload: map[string]*pb.Value{
			"consolidation_status": {Kind: &pb.Value_StringValue{StringValue: string(models.StatusConsolidated)}},
		},
	})
	if err != nil {
		return fmt.Errorf("qdrant mark consolidated: %w", err)
	}

	return nil
}

// UpdateDecay sets decay_factor for the given IDs.
func (q *QdrantStore) UpdateDecay(ctx context.Context, ids []string, decayFactor float64) error {
	pointIDs := make([]*pb.PointId, 0, len(ids))
	for _, id := range ids {
		pointIDs = append(pointIDs, &pb.PointId{PointIdOptions: &pb.PointId_Uuid{Uuid: id}})
	}

	_, err := q.points.SetPayload(ctx, &pb.SetPayloadPoints{
		CollectionName: q.cfg.Collection,
		PointsSelector: &pb.PointsSelector{
			PointsSelectorOneOf: &pb.PointsSelector_Points{
				Points: &pb.PointsIdsList{Ids: pointIDs},
			},
		},
		Payload: map[string]*pb.Value{
			"decay_factor": {Kind: &pb.Value_DoubleValue{DoubleValue: decayFactor}},
		},
	})
	if err != nil {
		return fmt.Errorf("qdrant update decay: %w", err)
	}

	return nil
}

// DeleteByIDs removes points by UUID.
func (q *QdrantStore) DeleteByIDs(ctx context.Context, ids []string) error {
	pointIDs := make([]*pb.PointId, 0, len(ids))
	for _, id := range ids {
		pointIDs = append(pointIDs, &pb.PointId{PointIdOptions: &pb.PointId_Uuid{Uuid: id}})
	}

	_, err := q.points.Delete(ctx, &pb.DeletePoints{
		CollectionName: q.cfg.Collection,
		Points: &pb.PointsSelector{
			PointsSelectorOneOf: &pb.PointsSelector_Points{
				Points: &pb.PointsIdsList{Ids: pointIDs},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("qdrant delete: %w", err)
	}

	return nil
}

// CountUnconsolidated returns the number of pending episodes for a user.
func (q *QdrantStore) CountUnconsolidated(ctx context.Context, userID string) (int, error) {
	resp, err := q.points.Count(ctx, &pb.CountPoints{
		CollectionName: q.cfg.Collection,
		Filter: &pb.Filter{
			Must: []*pb.Condition{
				{
					ConditionOneOf: &pb.Condition_Field{
						Field: &pb.FieldCondition{
							Key:   "user_id",
							Match: &pb.Match{MatchValue: &pb.Match_Keyword{Keyword: userID}},
						},
					},
				},
				{
					ConditionOneOf: &pb.Condition_Field{
						Field: &pb.FieldCondition{
							Key:   "consolidation_status",
							Match: &pb.Match{MatchValue: &pb.Match_Keyword{Keyword: string(models.StatusPending)}},
						},
					},
				},
			},
		},
		Exact: ptr(true),
	})
	if err != nil {
		return 0, fmt.Errorf("qdrant count: %w", err)
	}

	return int(resp.GetResult().GetCount()), nil
}

// Close releases the gRPC connection.
func (q *QdrantStore) Close() error {
	return q.conn.Close()
}

// --- Helpers ---

func payloadToEpisode(id string, payload map[string]*pb.Value) *models.Episode {
	ep := &models.Episode{
		ID:     id,
		UserID: getStringVal(payload, "user_id"),
		Content: getStringVal(payload, "content"),
		EventID: getStringVal(payload, "event_id"),
		MemoryType: models.MemoryType(getStringVal(payload, "memory_type")),
		ImportanceScore: getDoubleVal(payload, "importance_score"),
		ConsolidationStatus: models.ConsolidationStatus(getStringVal(payload, "consolidation_status")),
		SurprisalValue: getDoubleVal(payload, "surprisal_value"),
		DecayFactor: getDoubleVal(payload, "decay_factor"),
		TokenCount: int(getIntVal(payload, "token_count")),
	}

	ts := getIntVal(payload, "timestamp")
	if ts > 0 {
		ep.Timestamp = time.Unix(ts, 0)
	}

	if entList := payload["associated_entities"]; entList != nil {
		if lv := entList.GetListValue(); lv != nil {
			for _, v := range lv.GetValues() {
				ep.AssociatedEntities = append(ep.AssociatedEntities, v.GetStringValue())
			}
		}
	}

	return ep
}

func getStringVal(payload map[string]*pb.Value, key string) string {
	if v, ok := payload[key]; ok {
		return v.GetStringValue()
	}
	return ""
}

func getDoubleVal(payload map[string]*pb.Value, key string) float64 {
	if v, ok := payload[key]; ok {
		return v.GetDoubleValue()
	}
	return 0
}

func getIntVal(payload map[string]*pb.Value, key string) int64 {
	if v, ok := payload[key]; ok {
		return v.GetIntegerValue()
	}
	return 0
}

func ptr[T any](v T) *T {
	return &v
}
