package config

import (
	"os"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Monitor   MonitorConfig   `yaml:"monitor"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
	Resume    ResumeConfig    `yaml:"resume"`
	Logging   LoggingConfig   `yaml:"logging"`
}

type MonitorConfig struct {
	Processes          []ProcessPattern `yaml:"processes"`
	PollInterval       time.Duration    `yaml:"poll_interval"`
	TokenCheckInterval time.Duration    `yaml:"token_check_interval"`
	TerminalsDir       string           `yaml:"terminals_dir"`
	ClaudeProjectsDir  string           `yaml:"claude_projects_dir"`
}

type ProcessPattern struct {
	Name    string `yaml:"name"`
	Pattern string `yaml:"pattern"`
}

type RateLimitConfig struct {
	Provider      string             `yaml:"provider"`
	APIKey        string             `yaml:"api_key"`
	AuthToken     string             `yaml:"auth_token"`
	BaseURL       string             `yaml:"base_url"`
	EndpointPath  string             `yaml:"endpoint_path"`
	ProxyEndpoint string             `yaml:"proxy_endpoint"`
	Fallback      RateLimitFallback  `yaml:"fallback"`
	Headers       map[string]string  `yaml:"headers"`
}

type RateLimitFallback struct {
	LimitPer5H         int `yaml:"limit_per_5h"`
	ResetWindowMinutes int `yaml:"reset_window_minutes"`
}

type ResumeConfig struct {
	Strategy            string `yaml:"strategy"`
	RestartCommand      string `yaml:"restart_command"`
	SafetyMarginSeconds int    `yaml:"safety_margin_seconds"`
	SnapshotPath        string `yaml:"snapshot_path"`
}

type LoggingConfig struct {
	Level string `yaml:"level"`
	File  string `yaml:"file"`
}

var envPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	expanded := envPattern.ReplaceAllStringFunc(string(raw), func(match string) string {
		name := envPattern.FindStringSubmatch(match)[1]
		return os.Getenv(name)
	})

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, err
	}

	cfg.applyDefaults()
	return &cfg, nil
}

func (c *Config) applyDefaults() {
	if c.Monitor.PollInterval <= 0 {
		c.Monitor.PollInterval = 10 * time.Second
	}
	if c.Monitor.TokenCheckInterval <= 0 {
		c.Monitor.TokenCheckInterval = 30 * time.Second
	}
	if c.RateLimit.Fallback.LimitPer5H <= 0 {
		c.RateLimit.Fallback.LimitPer5H = 1_000_000
	}
	if c.RateLimit.Fallback.ResetWindowMinutes <= 0 {
		c.RateLimit.Fallback.ResetWindowMinutes = 300
	}
	if c.RateLimit.EndpointPath == "" {
		c.RateLimit.EndpointPath = "/v1/rate_limit"
	}
	if c.Resume.Strategy == "" {
		c.Resume.Strategy = "sigstop"
	}
	if c.Resume.SafetyMarginSeconds < 0 {
		c.Resume.SafetyMarginSeconds = 0
	}
	if c.Resume.SnapshotPath == "" {
		c.Resume.SnapshotPath = ".tokenresume-snapshots.json"
	}
	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
}
