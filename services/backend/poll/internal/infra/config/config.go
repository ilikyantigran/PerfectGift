package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config is loaded once at startup from the YAML file named by CONFIG_PATH.
// Data-only: addresses, ports, tuning knobs. Secrets (jwt_secret) come from the
// file only in local dev; in real deployments they are injected via env-expanded
// values or a secrets manager. values_local.yaml and values_docker.yaml share the
// same keys and differ only in values.
type Config struct {
	Service struct {
		Host     string `yaml:"host"`
		GrpcPort string `yaml:"grpc_port"`
		HttpPort string `yaml:"http_port"`
	} `yaml:"poll_service"`

	Postgres struct {
		DSN string `yaml:"dsn"`
	} `yaml:"postgres"`

	Valkey struct {
		Address string `yaml:"address"`
	} `yaml:"valkey"`

	NATS struct {
		URL     string `yaml:"url"`
		Stream  string `yaml:"stream"`  // JetStream stream name, e.g. POLL
		Subject string `yaml:"subject"` // publish subject, e.g. poll.completed
	} `yaml:"nats"`

	Auth struct {
		JWKSURL  string `yaml:"jwks_url"` // Identity's JWKS endpoint (EdDSA public keys)
		Issuer   string `yaml:"issuer"`   // expected `iss`
		Audience string `yaml:"audience"` // expected `aud`
	} `yaml:"auth"`

	Tokens struct {
		DefaultTTLSeconds int `yaml:"default_ttl_seconds"` // link token lifetime when request omits ttl
	} `yaml:"tokens"`

	RateLimit struct {
		PerTokenBudget int `yaml:"per_token_budget"` // submit attempts per window per token
		PerTokenWindow int `yaml:"per_token_window"` // seconds
		PerIPBudget    int `yaml:"per_ip_budget"`    // submit attempts per window per IP
		PerIPWindow    int `yaml:"per_ip_window"`    // seconds
	} `yaml:"ratelimit"`

	Web struct {
		AllowedOrigin string `yaml:"allowed_origin"` // CORS origin for the two public routes + link base
		LinkPath      string `yaml:"link_path"`      // path template for link_url, {token} substituted
	} `yaml:"web"`
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
