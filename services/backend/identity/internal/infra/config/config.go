package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config is loaded once at startup from the YAML file named by CONFIG_PATH.
// It is data-only: addresses, ports, TTLs, and non-secret provider identifiers.
// Signing key material and DB credentials are never checked in — private keys are
// generated in-process and DB creds live inside the DSN provided by the
// environment / secrets manager.
type Config struct {
	Service struct {
		Host     string `yaml:"host"`
		GrpcPort string `yaml:"grpc_port"`
		HttpPort string `yaml:"http_port"`
	} `yaml:"identity_service"`

	Postgres struct {
		DSN string `yaml:"dsn"`
	} `yaml:"postgres"`

	Valkey struct {
		Address string `yaml:"address"`
	} `yaml:"valkey"`

	Token struct {
		Issuer            string `yaml:"issuer"`
		Audience          string `yaml:"audience"`
		AccessTTLSeconds  int    `yaml:"access_ttl_seconds"`
		RefreshTTLSeconds int    `yaml:"refresh_ttl_seconds"`
	} `yaml:"token"`

	OAuth struct {
		GoogleClientIDs []string `yaml:"google_client_ids"`
		AppleClientIDs  []string `yaml:"apple_client_ids"`
	} `yaml:"oauth"`

	RateLimit struct {
		MaxAttempts   int `yaml:"max_attempts"`
		WindowSeconds int `yaml:"window_seconds"`
	} `yaml:"rate_limit"`
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
