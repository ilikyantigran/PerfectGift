package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is loaded once at startup from the YAML file named by CONFIG_PATH.
// Keep it data-only: addresses, ports, and tuning knobs. No secrets in the file —
// anything sensitive (APNs key, FCM credentials, DB password in the DSN) comes
// from the environment. The two config files (configs/values_local.yaml and
// configs/values_docker.yaml) must share the same keys and differ only in values.
type Config struct {
	Service struct {
		Host     string `yaml:"host"` // host the HTTP gateway dials its own gRPC server on
		GrpcPort string `yaml:"grpc_port"`
		HttpPort string `yaml:"http_port"`
	} `yaml:"notification_service"`

	// Postgres owns the notification schema (devices + outbox).
	Postgres struct {
		DSN string `yaml:"dsn"`
	} `yaml:"postgres"`

	// NATS JetStream durable consumer of PollCompleted / IdeasReady.
	NATS struct {
		URL              string `yaml:"url"`
		Stream           string `yaml:"stream"`
		Durable          string `yaml:"durable"`
		PollCompletedSub string `yaml:"poll_completed_subject"`
		IdeasReadySub    string `yaml:"ideas_ready_subject"`
	} `yaml:"nats"`

	// Dispatcher tuning: how often to sweep the outbox, batch size, retry policy,
	// and the claim lease (also the crash-recovery window).
	Dispatcher struct {
		Interval    time.Duration `yaml:"interval"`
		Batch       int           `yaml:"batch"`
		MaxAttempts int           `yaml:"max_attempts"`
		BaseBackoff time.Duration `yaml:"base_backoff"`
		Lease       time.Duration `yaml:"lease"`
	} `yaml:"dispatcher"`

	// APNs (iOS). Token-based auth (.p8 key). Paths/secrets come from env in prod.
	APNs struct {
		Enabled  bool   `yaml:"enabled"`
		KeyPath  string `yaml:"key_path"`
		KeyID    string `yaml:"key_id"`
		TeamID   string `yaml:"team_id"`
		Topic    string `yaml:"topic"` // app bundle id
		Sandbox  bool   `yaml:"sandbox"`
	} `yaml:"apns"`

	// FCM (Android).
	FCM struct {
		Enabled         bool   `yaml:"enabled"`
		CredentialsPath string `yaml:"credentials_path"`
		ProjectID       string `yaml:"project_id"`
	} `yaml:"fcm"`
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
