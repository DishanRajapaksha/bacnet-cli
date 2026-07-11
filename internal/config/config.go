package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	DefaultConfigPath = "config.yaml"
	DefaultPort       = 47808
)

var ErrConfig = errors.New("config error")

type Config struct {
	Connection ConnectionConfig `yaml:"connection"`
	Output     OutputConfig     `yaml:"output"`
}

type ConnectionConfig struct {
	Interface  string        `yaml:"interface,omitempty"`
	LocalIP    string        `yaml:"local_ip,omitempty"`
	Port       int           `yaml:"port"`
	SubnetCIDR int           `yaml:"subnet_cidr"`
	Timeout    time.Duration `yaml:"timeout"`
}

type OutputConfig struct {
	Format string `yaml:"format"`
}

type FileConfig struct {
	Config         `yaml:",inline"`
	DefaultProfile string            `yaml:"default_profile,omitempty"`
	Profiles       map[string]Config `yaml:"profiles,omitempty"`
}

type Overrides struct {
	Interface  string
	LocalIP    string
	Port       *int
	SubnetCIDR *int
	Timeout    *time.Duration
	Format     string
}

func DefaultConfig() Config {
	return Config{
		Connection: ConnectionConfig{
			Port:       DefaultPort,
			SubnetCIDR: 24,
			Timeout:    5 * time.Second,
		},
		Output: OutputConfig{Format: "table"},
	}
}

func LoadForProfile(path string, profile string, overrides Overrides) (Config, error) {
	cfg := DefaultConfig()
	if path != "" {
		file, err := LoadFile(path)
		if err != nil {
			return cfg, err
		}
		cfg = mergeConfig(cfg, file.Config)
		selected := profile
		if selected == "" {
			selected = file.DefaultProfile
		}
		if selected != "" {
			profileCfg, ok := file.Profiles[selected]
			if !ok {
				return cfg, fmt.Errorf("%w: profile %q not found", ErrConfig, selected)
			}
			cfg = mergeConfig(cfg, profileCfg)
		}
	}
	applyOverrides(&cfg, overrides)
	if err := Validate(cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func LoadFile(path string) (FileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && path == DefaultConfigPath {
			return FileConfig{Config: DefaultConfig()}, nil
		}
		return FileConfig{}, fmt.Errorf("%w: read config %q: %v", ErrConfig, path, err)
	}
	cfg := FileConfig{Config: DefaultConfig()}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return FileConfig{}, fmt.Errorf("%w: parse config %q: %v", ErrConfig, path, err)
	}
	return cfg, nil
}

func Validate(cfg Config) error {
	if cfg.Connection.Interface != "" && cfg.Connection.LocalIP != "" {
		return fmt.Errorf("%w: set connection.interface or connection.local_ip, not both", ErrConfig)
	}
	if cfg.Connection.Port < 1 || cfg.Connection.Port > 65535 {
		return fmt.Errorf("%w: connection.port must be between 1 and 65535", ErrConfig)
	}
	if cfg.Connection.SubnetCIDR < 0 || cfg.Connection.SubnetCIDR > 32 {
		return fmt.Errorf("%w: connection.subnet_cidr must be between 0 and 32", ErrConfig)
	}
	if cfg.Connection.Timeout <= 0 {
		return fmt.Errorf("%w: connection.timeout must be greater than zero", ErrConfig)
	}
	switch strings.ToLower(strings.TrimSpace(cfg.Output.Format)) {
	case "table", "text", "json", "jsonl", "csv":
	default:
		return fmt.Errorf("%w: output.format must be table, text, json, jsonl, or csv", ErrConfig)
	}
	return nil
}

func StarterConfigYAML() ([]byte, error) {
	cfg := DefaultConfig()
	return yaml.Marshal(FileConfig{
		Config:         cfg,
		DefaultProfile: "local",
		Profiles: map[string]Config{
			"local": {
				Connection: ConnectionConfig{Interface: "en0", Port: DefaultPort, SubnetCIDR: 24, Timeout: 5 * time.Second},
			},
			"linux": {
				Connection: ConnectionConfig{Interface: "eth0", Port: DefaultPort, SubnetCIDR: 24, Timeout: 5 * time.Second},
			},
		},
	})
}

func mergeConfig(base Config, override Config) Config {
	if override.Connection.Interface != "" {
		base.Connection.Interface = override.Connection.Interface
		base.Connection.LocalIP = ""
	}
	if override.Connection.LocalIP != "" {
		base.Connection.LocalIP = override.Connection.LocalIP
		base.Connection.Interface = ""
	}
	if override.Connection.Port != 0 {
		base.Connection.Port = override.Connection.Port
	}
	if override.Connection.SubnetCIDR != 0 {
		base.Connection.SubnetCIDR = override.Connection.SubnetCIDR
	}
	if override.Connection.Timeout != 0 {
		base.Connection.Timeout = override.Connection.Timeout
	}
	if override.Output.Format != "" {
		base.Output.Format = override.Output.Format
	}
	return base
}

func applyOverrides(cfg *Config, overrides Overrides) {
	if overrides.Interface != "" {
		cfg.Connection.Interface = overrides.Interface
		cfg.Connection.LocalIP = ""
	}
	if overrides.LocalIP != "" {
		cfg.Connection.LocalIP = overrides.LocalIP
		cfg.Connection.Interface = ""
	}
	if overrides.Port != nil {
		cfg.Connection.Port = *overrides.Port
	}
	if overrides.SubnetCIDR != nil {
		cfg.Connection.SubnetCIDR = *overrides.SubnetCIDR
	}
	if overrides.Timeout != nil {
		cfg.Connection.Timeout = *overrides.Timeout
	}
	if overrides.Format != "" {
		cfg.Output.Format = overrides.Format
	}
}
