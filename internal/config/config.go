package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/DishanRajapaksha/bacnet-cli/internal/bacnetclient"
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
	Devices    []DeviceConfig   `yaml:"devices,omitempty"`
	Points     []PointConfig    `yaml:"points,omitempty"`
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

type DeviceConfig struct {
	Name         string  `yaml:"name"`
	DeviceID     int     `yaml:"device_id"`
	Address      string  `yaml:"address,omitempty"`
	Port         int     `yaml:"port,omitempty"`
	Network      int     `yaml:"network,omitempty"`
	MSTPMAC      *int    `yaml:"mstp_mac,omitempty"`
	MaxAPDU      uint32  `yaml:"max_apdu,omitempty"`
	Segmentation *uint32 `yaml:"segmentation,omitempty"`
}

type PointConfig struct {
	Name       string  `yaml:"name"`
	Device     string  `yaml:"device"`
	Object     string  `yaml:"object"`
	Property   string  `yaml:"property,omitempty"`
	ArrayIndex *uint32 `yaml:"array_index,omitempty"`
	Type       string  `yaml:"type,omitempty"`
	Unit       string  `yaml:"unit,omitempty"`
	Writable   bool    `yaml:"writable,omitempty"`
	Priority   uint8   `yaml:"priority,omitempty"`
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
	if err := validateDeclaredLists(cfg); err != nil {
		return FileConfig{}, err
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
	if err := validateDevices(cfg.Devices); err != nil {
		return err
	}
	if err := validatePointsSyntax(cfg.Points); err != nil {
		return err
	}
	devices := make(map[string]struct{}, len(cfg.Devices))
	for _, device := range cfg.Devices {
		devices[device.Name] = struct{}{}
	}
	for _, point := range cfg.Points {
		if _, ok := devices[point.Device]; !ok {
			return fmt.Errorf("%w: point %q references unknown device %q", ErrConfig, point.Name, point.Device)
		}
	}
	return nil
}

func StarterConfigYAML() ([]byte, error) {
	cfg := DefaultConfig()
	segmentation := uint32(3)
	cfg.Devices = []DeviceConfig{
		{Name: "ahu", DeviceID: 1234, Port: DefaultPort, MaxAPDU: 1476, Segmentation: &segmentation},
	}
	cfg.Points = []PointConfig{
		{Name: "supply_air_temperature", Device: "ahu", Object: "analog-input:1", Property: "present-value", Unit: "°C"},
		{Name: "cooling_setpoint", Device: "ahu", Object: "analog-value:1", Property: "present-value", Type: "float32", Unit: "°C", Writable: true, Priority: 16},
	}
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
	if len(override.Devices) > 0 {
		base.Devices = mergeDevices(base.Devices, override.Devices)
	}
	if len(override.Points) > 0 {
		base.Points = mergePoints(base.Points, override.Points)
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

func mergeDevices(base []DeviceConfig, override []DeviceConfig) []DeviceConfig {
	out := append([]DeviceConfig(nil), base...)
	index := make(map[string]int, len(out))
	for i, device := range out {
		index[device.Name] = i
	}
	for _, device := range override {
		if i, ok := index[device.Name]; ok && device.Name != "" {
			out[i] = device
			continue
		}
		index[device.Name] = len(out)
		out = append(out, device)
	}
	return out
}

func mergePoints(base []PointConfig, override []PointConfig) []PointConfig {
	out := append([]PointConfig(nil), base...)
	index := make(map[string]int, len(out))
	for i, point := range out {
		index[point.Name] = i
	}
	for _, point := range override {
		if i, ok := index[point.Name]; ok && point.Name != "" {
			out[i] = point
			continue
		}
		index[point.Name] = len(out)
		out = append(out, point)
	}
	return out
}

func validateDevices(devices []DeviceConfig) error {
	seen := map[string]struct{}{}
	for _, device := range devices {
		name := strings.TrimSpace(device.Name)
		if name == "" {
			return fmt.Errorf("%w: device name is required", ErrConfig)
		}
		if _, ok := seen[name]; ok {
			return fmt.Errorf("%w: duplicate device name %q", ErrConfig, name)
		}
		seen[name] = struct{}{}
		if device.DeviceID < 0 || device.DeviceID > 4194303 {
			return fmt.Errorf("%w: device %q device_id must be between 0 and 4194303", ErrConfig, name)
		}
		if device.Address != "" && net.ParseIP(device.Address) == nil {
			return fmt.Errorf("%w: device %q address must be an IP address", ErrConfig, name)
		}
		if device.Port < 0 || device.Port > 65535 {
			return fmt.Errorf("%w: device %q port must be between 1 and 65535 when set", ErrConfig, name)
		}
		if device.Network < 0 || device.Network > 65535 {
			return fmt.Errorf("%w: device %q network must be between 0 and 65535", ErrConfig, name)
		}
		if device.MSTPMAC != nil && (*device.MSTPMAC < 0 || *device.MSTPMAC > 255) {
			return fmt.Errorf("%w: device %q mstp_mac must be between 0 and 255", ErrConfig, name)
		}
		if device.MaxAPDU > 1476 {
			return fmt.Errorf("%w: device %q max_apdu must be 1476 or less", ErrConfig, name)
		}
		if device.Segmentation != nil && *device.Segmentation > 3 {
			return fmt.Errorf("%w: device %q segmentation must be between 0 and 3", ErrConfig, name)
		}
	}
	return nil
}

func validatePointsSyntax(points []PointConfig) error {
	seen := map[string]struct{}{}
	for _, point := range points {
		name := strings.TrimSpace(point.Name)
		if name == "" {
			return fmt.Errorf("%w: point name is required", ErrConfig)
		}
		if _, ok := seen[name]; ok {
			return fmt.Errorf("%w: duplicate point name %q", ErrConfig, name)
		}
		seen[name] = struct{}{}
		if strings.TrimSpace(point.Device) == "" {
			return fmt.Errorf("%w: point %q device is required", ErrConfig, name)
		}
		if _, err := bacnetclient.ParseObjectIdentifier(point.Object); err != nil {
			return fmt.Errorf("%w: point %q: %v", ErrConfig, name, err)
		}
		property := point.Property
		if property == "" {
			property = "present-value"
		}
		if _, err := bacnetclient.ParsePropertyIdentifier(property); err != nil {
			return fmt.Errorf("%w: point %q: %v", ErrConfig, name, err)
		}
		switch strings.ToLower(strings.TrimSpace(point.Type)) {
		case "", "string", "bool", "boolean", "uint", "unsigned", "enumerated", "int", "signed", "float", "real", "float32", "double", "float64":
		default:
			return fmt.Errorf("%w: point %q has unsupported type %q", ErrConfig, name, point.Type)
		}
		if point.Writable && strings.TrimSpace(point.Type) == "" {
			return fmt.Errorf("%w: writable point %q requires type", ErrConfig, name)
		}
		if point.Priority > 16 {
			return fmt.Errorf("%w: point %q priority must be between 1 and 16 when set", ErrConfig, name)
		}
	}
	return nil
}

func validateDeclaredLists(file FileConfig) error {
	if err := validateDevices(file.Devices); err != nil {
		return err
	}
	if err := validatePointsSyntax(file.Points); err != nil {
		return err
	}
	for name, profile := range file.Profiles {
		if err := validateDevices(profile.Devices); err != nil {
			return fmt.Errorf("%w: profile %q: %v", ErrConfig, name, err)
		}
		if err := validatePointsSyntax(profile.Points); err != nil {
			return fmt.Errorf("%w: profile %q: %v", ErrConfig, name, err)
		}
	}
	return nil
}
