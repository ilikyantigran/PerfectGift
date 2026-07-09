// Package config loads the gateway configuration once at startup from the YAML file
// named by CONFIG_PATH. It is data-only (addresses, ports, budgets); secrets are not
// checked in. The two files configs/values_local.yaml and configs/values_docker.yaml
// share the same keys and differ only in values.
package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Service struct {
		// The gateway is HTTP-only (it exposes no gRPC server of its own — it is a
		// gRPC *client* to the domain services), so there is a single HTTP port.
		Host     string `yaml:"host"`
		HttpPort string `yaml:"http_port"`
	} `yaml:"gateway_service"`

	// gRPC addresses of the five downstream domain services.
	Downstreams struct {
		Identity     string `yaml:"identity"`
		Poll         string `yaml:"poll"`
		Surprise     string `yaml:"surprise"`
		Catalog      string `yaml:"catalog"`
		Notification string `yaml:"notification"`
	} `yaml:"downstreams"`

	// Local JWT validation via Identity's JWKS (no per-request call on the hot path).
	Auth struct {
		JWKSURL     string        `yaml:"jwks_url"`
		Issuer      string        `yaml:"issuer"`
		Audience    string        `yaml:"audience"`
		JWKSRefresh time.Duration `yaml:"jwks_refresh"`
	} `yaml:"auth"`

	// CORS is allowed ONLY for the Poll Web Page origin(s), on the anonymous poll routes.
	CORS struct {
		PollOrigins []string `yaml:"poll_origins"`
	} `yaml:"cors"`

	// Rate-limit budgets (requests per minute). 0 disables a given limiter.
	// In production these counters are intended to live in Valkey (the only stateful
	// bit); the in-process limiter is the default and needs no external state.
	RateLimit struct {
		GlobalPerMin  int    `yaml:"global_per_min"`
		PerIPPerMin   int    `yaml:"per_ip_per_min"`
		PerUserPerMin int    `yaml:"per_user_per_min"`
		RefreshPerMin int    `yaml:"refresh_per_min"`
		ValkeyAddr    string `yaml:"valkey_addr"` // optional; empty → in-process counters
	} `yaml:"ratelimit"`
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
