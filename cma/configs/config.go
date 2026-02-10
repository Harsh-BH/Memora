package configs

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration structure for the CMA backend.
type Config struct {
	Server        ServerConfig        `yaml:"server"`
	Qdrant        QdrantConfig        `yaml:"qdrant"`
	Neo4j         Neo4jConfig         `yaml:"neo4j"`
	Redis         RedisConfig         `yaml:"redis"`
	LLM           LLMConfig           `yaml:"llm"`
	Segmentation  SegmentationConfig  `yaml:"segmentation"`
	Knapsack      KnapsackConfig      `yaml:"knapsack"`
	DIG           DIGConfig           `yaml:"dig"`
	Consolidation ConsolidationConfig `yaml:"consolidation"`
	Retrieval     RetrievalConfig     `yaml:"retrieval"`
	Metrics       MetricsConfig       `yaml:"metrics"`
}

type ServerConfig struct {
	Host         string        `yaml:"host"`
	Port         int           `yaml:"port"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
}

type QdrantConfig struct {
	Host       string `yaml:"host"`
	GRPCPort   int    `yaml:"grpc_port"`
	Collection string `yaml:"collection"`
	VectorSize uint64 `yaml:"vector_size"`
	HnswM      uint64 `yaml:"hnsw_m"`
	HnswEF     uint64 `yaml:"hnsw_ef"`
}

type Neo4jConfig struct {
	URI      string `yaml:"uri"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
}

type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

type LLMConfig struct {
	Provider       string  `yaml:"provider"`
	APIKey         string  `yaml:"api_key"`
	Model          string  `yaml:"model"`
	EmbeddingModel string  `yaml:"embedding_model"`
	MaxTokens      int     `yaml:"max_tokens"`
	Temperature    float64 `yaml:"temperature"`
}

type SegmentationConfig struct {
	Gamma            float64 `yaml:"gamma"`
	WindowSize       int     `yaml:"window_size"`
	MinEpisodeTokens int     `yaml:"min_episode_tokens"`
	MaxEpisodeTokens int     `yaml:"max_episode_tokens"`
}

type KnapsackConfig struct {
	TokenBudget      int     `yaml:"token_budget"`
	ForceRecentTurns int     `yaml:"force_recent_turns"`
	LambdaInit       float64 `yaml:"lambda_init"`
}

type DIGConfig struct {
	MinScore        float64 `yaml:"min_score"`
	FallbackEnabled bool    `yaml:"fallback_enabled"`
}

type ConsolidationConfig struct {
	InactivityTimeout  time.Duration `yaml:"inactivity_timeout"`
	MaxUnconsolidated  int           `yaml:"max_unconsolidated"`
	DBSCANEpsilon      float64       `yaml:"dbscan_epsilon"`
	DBSCANMinPoints    int           `yaml:"dbscan_min_points"`
	DecayRate          float64       `yaml:"decay_rate"`
	WorkerConcurrency  int           `yaml:"worker_concurrency"`
	CheckInterval      time.Duration `yaml:"check_interval"`
}

type RetrievalConfig struct {
	VectorTopK   int           `yaml:"vector_top_k"`
	GraphMaxHops int           `yaml:"graph_max_hops"`
	Timeout      time.Duration `yaml:"timeout"`
}

type MetricsConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}

// Load reads the configuration from the given file path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Expand environment variables in the config.
	expanded := os.ExpandEnv(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, err
	}

	cfg.applyDefaults()
	return &cfg, nil
}

func (c *Config) applyDefaults() {
	if c.Server.Port == 0 {
		c.Server.Port = 8080
	}
	if c.Server.ReadTimeout == 0 {
		c.Server.ReadTimeout = 30 * time.Second
	}
	if c.Server.WriteTimeout == 0 {
		c.Server.WriteTimeout = 30 * time.Second
	}
	if c.Qdrant.Collection == "" {
		c.Qdrant.Collection = "cma_episodes"
	}
	if c.Qdrant.VectorSize == 0 {
		c.Qdrant.VectorSize = 1536
	}
	if c.Segmentation.Gamma == 0 {
		c.Segmentation.Gamma = 1.5
	}
	if c.Segmentation.WindowSize == 0 {
		c.Segmentation.WindowSize = 50
	}
	if c.Knapsack.TokenBudget == 0 {
		c.Knapsack.TokenBudget = 4096
	}
	if c.Knapsack.ForceRecentTurns == 0 {
		c.Knapsack.ForceRecentTurns = 3
	}
	if c.Consolidation.InactivityTimeout == 0 {
		c.Consolidation.InactivityTimeout = 15 * time.Minute
	}
	if c.Consolidation.MaxUnconsolidated == 0 {
		c.Consolidation.MaxUnconsolidated = 10
	}
	if c.Consolidation.WorkerConcurrency == 0 {
		c.Consolidation.WorkerConcurrency = 5
	}
	if c.Consolidation.CheckInterval == 0 {
		c.Consolidation.CheckInterval = 1 * time.Minute
	}
	if c.Retrieval.VectorTopK == 0 {
		c.Retrieval.VectorTopK = 20
	}
	if c.Retrieval.GraphMaxHops == 0 {
		c.Retrieval.GraphMaxHops = 2
	}
	if c.Retrieval.Timeout == 0 {
		c.Retrieval.Timeout = 10 * time.Second
	}
}
