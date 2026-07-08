package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config is loaded once at startup from the YAML file named by CONFIG_PATH.
// Keep it data-only: addresses, ports, and tuning knobs. No secrets in the file —
// anything sensitive (e.g. the embedding API key) comes from the environment.
// The two config files (configs/values_local.yaml and configs/values_docker.yaml)
// must share the same keys and differ only in values.
type Config struct {
	Service struct {
		Host     string `yaml:"host"` // host the HTTP gateway dials its own gRPC server on
		GrpcPort string `yaml:"grpc_port"`
		HttpPort string `yaml:"http_port"`
	} `yaml:"catalog_service"`

	// Postgres owns the catalog schema (reference data + curated corpus + pgvector).
	Postgres struct {
		DSN string `yaml:"dsn"`
	} `yaml:"postgres"`

	// Valkey caches hot reference reads (holidays/categories/budget bands).
	Valkey struct {
		Address string `yaml:"address"`
	} `yaml:"valkey"`

	// Embedding pins the model + vector dimension. These MUST match what the
	// Surprise service uses to embed queries, or similarity search is meaningless.
	// APIKeyEnv names the environment variable holding the key (never the key itself).
	// When Endpoint is empty the service uses the deterministic fake embedder, so
	// it can boot and be exercised without an external embedding API.
	Embedding struct {
		Model     string `yaml:"model"`
		Dimension int    `yaml:"dimension"`
		Endpoint  string `yaml:"endpoint"`
		APIKeyEnv string `yaml:"api_key_env"`
	} `yaml:"embedding"`

	// Catalog tuning knobs.
	Catalog struct {
		ReferenceCacheTTLSeconds int `yaml:"reference_cache_ttl_seconds"` // TTL for cached reference reads
		DefaultTopK              int `yaml:"default_top_k"`               // SearchInspiration default when unset
		MaxTopK                  int `yaml:"max_top_k"`                   // SearchInspiration hard cap
	} `yaml:"catalog"`
}

// InitConfig opens the YAML file at path and decodes it into a Config.
func InitConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	c := &Config{}
	if err = yaml.NewDecoder(f).Decode(c); err != nil {
		return nil, err
	}
	return c, nil
}
