package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is loaded once at startup from the YAML file named by CONFIG_PATH.
// Keep it data-only: addresses, ports, and tuning knobs. The two config files
// (configs/values_local.yaml and configs/values_docker.yaml) must share the
// same keys and differ only in values.
type Config struct {
	Service struct {
		HTTPPort string `yaml:"http_port"` // HTTP port for the ingest/query API + web UI
	} `yaml:"log_service"`

	// SQLite store. Path points at a file that can live on a Docker volume.
	Store struct {
		Path string `yaml:"path"`
	} `yaml:"store"`

	// Retention pruner: delete rows older than Window, sweeping every Interval.
	Retention struct {
		Window   time.Duration `yaml:"window"`
		Interval time.Duration `yaml:"interval"`
	} `yaml:"retention"`
}

// InitConfig opens the YAML file at path and decodes it into a Config,
// applying sane defaults for any zero-valued knob.
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
	c.applyDefaults()
	return c, nil
}

func (c *Config) applyDefaults() {
	if c.Service.HTTPPort == "" {
		c.Service.HTTPPort = "8086"
	}
	if c.Store.Path == "" {
		c.Store.Path = "/data/logs.db"
	}
	if c.Retention.Window == 0 {
		c.Retention.Window = 72 * time.Hour
	}
	if c.Retention.Interval == 0 {
		c.Retention.Interval = 10 * time.Minute
	}
}
