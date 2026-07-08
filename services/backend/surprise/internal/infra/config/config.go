package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is loaded once at startup from the YAML file named by CONFIG_PATH.
// Data-only: addresses, ports, and tuning knobs. Secrets (the Anthropic API key)
// come from the environment, never the file. The two config files
// (values_local.yaml / values_docker.yaml) share keys and differ only in values.
type Config struct {
	Service struct {
		Host     string `yaml:"host"`
		GrpcPort string `yaml:"grpc_port"`
		HttpPort string `yaml:"http_port"`
	} `yaml:"surprise_service"`

	Postgres struct {
		DSN string `yaml:"dsn"`
	} `yaml:"postgres"`

	Valkey struct {
		Address string `yaml:"address"`
	} `yaml:"valkey"`

	NATS struct {
		URL            string `yaml:"url"`
		Stream         string `yaml:"stream"`
		RequestSubject string `yaml:"request_subject"`
		ReadySubject   string `yaml:"ready_subject"`
		DurableName    string `yaml:"durable_name"`
	} `yaml:"nats"`

	Anthropic struct {
		BaseURL        string `yaml:"base_url"`
		Version        string `yaml:"version"`
		SonnetModel    string `yaml:"sonnet_model"`
		OpusModel      string `yaml:"opus_model"`
		HaikuModel     string `yaml:"haiku_model"`
		EmbeddingModel string `yaml:"embedding_model"`
		EmbeddingDim   int    `yaml:"embedding_dim"`
		MaxTokens      int    `yaml:"max_tokens"`
		// APIKey is NOT read from YAML — it is injected from ANTHROPIC_API_KEY.
		APIKey string `yaml:"-"`
	} `yaml:"anthropic"`

	Downstreams struct {
		Poll    string `yaml:"poll"`
		Catalog string `yaml:"catalog"`
	} `yaml:"downstreams"`

	Worker struct {
		PoolSize    int `yaml:"pool_size"`
		IdeasWanted int `yaml:"ideas_wanted"`
	} `yaml:"worker"`

	Resilience struct {
		LLMTimeout          time.Duration `yaml:"llm_timeout"`
		RetryMaxAttempts    int           `yaml:"retry_max_attempts"`
		RetryBaseBackoff    time.Duration `yaml:"retry_base_backoff"`
		BreakerMaxFailures  int           `yaml:"breaker_max_failures"`
		BreakerOpenDuration time.Duration `yaml:"breaker_open_duration"`
	} `yaml:"resilience"`

	Cache struct {
		StatusTTL      time.Duration `yaml:"status_ttl"`
		IdempotencyTTL time.Duration `yaml:"idempotency_ttl"`
		LLMTTL         time.Duration `yaml:"llm_ttl"`
	} `yaml:"cache"`
}

// InitConfig opens the YAML file at path, decodes it, then overlays secrets from
// the environment and applies defaults.
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
	c.Anthropic.APIKey = os.Getenv("ANTHROPIC_API_KEY")
	c.applyDefaults()
	return c, nil
}

func (c *Config) applyDefaults() {
	if c.Anthropic.BaseURL == "" {
		c.Anthropic.BaseURL = "https://api.anthropic.com"
	}
	if c.Anthropic.Version == "" {
		c.Anthropic.Version = "2023-06-01"
	}
	if c.Anthropic.SonnetModel == "" {
		c.Anthropic.SonnetModel = "claude-sonnet-5"
	}
	if c.Anthropic.OpusModel == "" {
		c.Anthropic.OpusModel = "claude-opus-4-8"
	}
	if c.Anthropic.HaikuModel == "" {
		c.Anthropic.HaikuModel = "claude-haiku-4-5"
	}
	if c.Anthropic.EmbeddingDim == 0 {
		c.Anthropic.EmbeddingDim = 1536
	}
	if c.Anthropic.MaxTokens == 0 {
		c.Anthropic.MaxTokens = 2048
	}
	if c.Worker.PoolSize == 0 {
		c.Worker.PoolSize = 4
	}
	if c.Worker.IdeasWanted == 0 {
		c.Worker.IdeasWanted = 5
	}
	if c.Resilience.LLMTimeout == 0 {
		c.Resilience.LLMTimeout = 30 * time.Second
	}
	if c.Resilience.RetryMaxAttempts == 0 {
		c.Resilience.RetryMaxAttempts = 3
	}
	if c.Resilience.RetryBaseBackoff == 0 {
		c.Resilience.RetryBaseBackoff = 200 * time.Millisecond
	}
	if c.Resilience.BreakerMaxFailures == 0 {
		c.Resilience.BreakerMaxFailures = 5
	}
	if c.Resilience.BreakerOpenDuration == 0 {
		c.Resilience.BreakerOpenDuration = 30 * time.Second
	}
	if c.Cache.StatusTTL == 0 {
		c.Cache.StatusTTL = time.Hour
	}
	if c.Cache.IdempotencyTTL == 0 {
		c.Cache.IdempotencyTTL = 24 * time.Hour
	}
	if c.Cache.LLMTTL == 0 {
		c.Cache.LLMTTL = 24 * time.Hour
	}
}
