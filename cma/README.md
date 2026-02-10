# Continuum Memory Architecture (CMA) Backend

Production-grade cognitive memory operating system built in Go, implementing the Continuum Memory Architecture as described in *"Building a Cognitive AI: The Continuum Memory Architecture"*.

## Architecture Overview

```
┌─────────────────────────────────────────────────────┐
│                   WAKE MODE (API)                   │
│  ┌──────────┐  ┌───────────┐  ┌──────────────────┐ │
│  │  Ingest   │  │ Retrieval │  │    Workspace     │ │
│  │ Pipeline  │  │  Hybrid   │  │  DIG + Knapsack  │ │
│  │ Surprisal │  │Vec + Graph│  │  Context Assembly │ │
│  └────┬─────┘  └─────┬─────┘  └────────┬─────────┘ │
│       │              │                  │           │
│  ┌────▼──────────────▼──────────────────▼─────────┐ │
│  │              Memory Stores                     │ │
│  │  ┌─────────────┐        ┌──────────────────┐   │ │
│  │  │   Qdrant    │        │     Neo4j        │   │ │
│  │  │  (Episodic) │        │   (Semantic)     │   │ │
│  │  │ Hippocampus │        │   Neocortex      │   │ │
│  │  └─────────────┘        └──────────────────┘   │ │
│  └────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────┐
│                 SLEEP MODE (Workers)                │
│  ┌──────────┐  ┌───────────┐  ┌──────────────────┐ │
│  │ DBSCAN   │→ │   LLM     │→ │   Conflict       │ │
│  │ Cluster  │  │ Synthesis │  │   Resolution     │ │
│  │          │  │ + Triples │  │   + Graph Write  │ │
│  └──────────┘  └───────────┘  └──────────────────┘ │
│  Asynq Workers │ Redis Locks │ Temporal Decay      │
└─────────────────────────────────────────────────────┘
```

## Prerequisites

- Go 1.22+
- Docker & Docker Compose

## Quick Start

### 1. Start Infrastructure

```bash
cd cma
docker compose up -d
```

This starts:
- **Qdrant** (vector store) on ports 6333/6334
- **Neo4j** (graph store) on ports 7474/7687
- **Redis** on port 6379
- **Asynq Dashboard** on port 8980

### 2. Configure

```bash
export OPENAI_API_KEY="your-api-key"
```

Edit `configs/config.yaml` for custom parameters.

### 3. Build and Run

```bash
go mod tidy
go build -o cma-server ./cmd/api
./cma-server
```

The server starts on `http://localhost:8080`.

## API Endpoints

### Health Check

```bash
curl http://localhost:8080/health
```

### Ingest Memory (Episodic Write)

```bash
# Ingest user message
curl -X POST http://localhost:8080/api/v1/ingest \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user_123",
    "content": "I just started a new job at Google as a senior engineer. I moved to Mountain View last week.",
    "role": "user"
  }'

# Ingest assistant response
curl -X POST http://localhost:8080/api/v1/ingest \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user_123",
    "content": "Congratulations on the new role at Google! How are you finding Mountain View so far?",
    "role": "assistant"
  }'
```

### Query Memory (Cognitive Workspace Read)

```bash
curl -X POST http://localhost:8080/api/v1/query \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user_123",
    "query": "Where does the user work?",
    "token_budget": 4096
  }'
```

### Trigger Consolidation (Admin)

```bash
curl -X POST "http://localhost:8080/api/v1/admin/consolidate?user_id=user_123"
```

### Prometheus Metrics

```bash
curl http://localhost:8080/metrics
```

## Project Structure

```
cma/
├── cmd/api/main.go                    # Gin server, DI, graceful shutdown
├── internal/
│   ├── models/models.go               # Domain types (Episode, Triple, etc.)
│   ├── ingest/service.go              # Ingest pipeline (surprisal → Qdrant)
│   ├── workspace/workspace.go         # Cognitive workspace (full read path)
│   ├── retrieval/service.go           # Concurrent hybrid retrieval
│   ├── consolidation/
│   │   ├── worker.go                  # Asynq Sleep cycle worker
│   │   ├── scheduler.go              # Periodic trigger + Redis locks
│   │   ├── clustering.go             # DBSCAN over embeddings
│   │   └── conflict.go               # Temporal decay conflict resolution
│   ├── vectorstore/
│   │   ├── vectorstore.go            # VectorStore interface
│   │   └── qdrant.go                 # Qdrant gRPC implementation
│   ├── graphstore/
│   │   ├── graphstore.go             # GraphStore interface
│   │   └── neo4j.go                  # Neo4j implementation
│   ├── llm/
│   │   ├── llm.go                    # LLM Provider interface
│   │   └── openai.go                 # OpenAI implementation
│   ├── segmentation/surprisal.go     # Bayesian Surprise segmentation
│   ├── dig/dig.go                    # DIG reranking
│   ├── knapsack/knapsack.go          # Lagrangian relaxation optimizer
│   ├── middleware/middleware.go       # Gin middleware stack
│   └── metrics/metrics.go            # Prometheus instrumentation
├── pkg/utils.go                       # Shared utilities
├── configs/
│   ├── config.go                      # Config loader
│   └── config.yaml                    # Runtime configuration
├── docker-compose.yml                 # Infrastructure services
└── go.mod                             # Go module
```

## Key Mathematical Formulations

| Component      | Formula                                     | Implementation                    |
|---------------|---------------------------------------------|-----------------------------------|
| Surprisal     | `S(x_t) = -log P(x_t \| x_<t)`             | `segmentation/surprisal.go`       |
| Boundary      | `S > μ + γσ`                                | Rolling window per user           |
| DIG           | `DIG(d\|x) = log P(y\|x,d) - log P(y\|x)` | `dig/dig.go`                      |
| Knapsack      | `x_i = 1 iff v_i/w_i ≥ λ`                  | `knapsack/knapsack.go`            |
| Conflict Decay| `confidence *= decay_rate`                  | `consolidation/conflict.go`       |

## Biological Analogies

| CMA Component      | Biological Analogy | Implementation          |
|--------------------|-------------------|------------------------|
| Qdrant (Episodes)  | Hippocampus        | Fast episodic encoding |
| Neo4j (Knowledge)  | Neocortex          | Slow semantic learning |
| Consolidation      | Sleep cycles       | Asynq worker pool      |
| Workspace          | Working memory     | Context assembly       |
| DIG + Knapsack     | Attention          | Information selection  |

## Configuration Reference

Key parameters in `configs/config.yaml`:

- `segmentation.gamma`: Surprise threshold sensitivity (default: 1.5)
- `knapsack.token_budget`: Context window budget (default: 4096)
- `consolidation.inactivity_timeout`: Sleep trigger timeout (default: 15m)
- `consolidation.max_unconsolidated`: Episode count trigger (default: 10)
- `consolidation.decay_rate`: Conflict temporal decay (default: 0.95)

## Neo4j Schema Migration

The schema is auto-created on startup. Manual migration if needed:

```cypher
CREATE CONSTRAINT entity_id IF NOT EXISTS FOR (e:Entity) REQUIRE e.id IS UNIQUE;
CREATE CONSTRAINT concept_id IF NOT EXISTS FOR (c:Concept) REQUIRE c.id IS UNIQUE;
CREATE CONSTRAINT event_id IF NOT EXISTS FOR (ev:Event) REQUIRE ev.id IS UNIQUE;
CREATE INDEX entity_name IF NOT EXISTS FOR (e:Entity) ON (e.name);
CREATE INDEX entity_user IF NOT EXISTS FOR (e:Entity) ON (e.user_id);
```

## Multi-Tenant Support

Set the `X-Tenant-ID` header for tenant-scoped requests. All data is partitioned by `user_id` in both Qdrant payloads and Neo4j node properties.
