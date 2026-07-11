package config

import (
	"testing"
	"time"
)

func TestDefaultConfigIsValid(t *testing.T) {
	if err := Validate(DefaultConfig()); err != nil {
		t.Fatal(err)
	}
}

func TestRejectsInterfaceAndLocalIP(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Connection.Interface = "eth0"
	cfg.Connection.LocalIP = "192.0.2.10"
	if err := Validate(cfg); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestRejectsNonPositiveTimeout(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Connection.Timeout = -time.Second
	if err := Validate(cfg); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestNamedDeviceAndPointAreValid(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Devices = []DeviceConfig{{Name: "ahu", DeviceID: 100}}
	cfg.Points = []PointConfig{{
		Name: "temperature", Device: "ahu", Object: "analog-input:1", Property: "present-value", Unit: "°C",
	}}
	if err := Validate(cfg); err != nil {
		t.Fatal(err)
	}
}

func TestPointMustReferenceKnownDevice(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Points = []PointConfig{{Name: "temperature", Device: "missing", Object: "analog-input:1"}}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestWritablePointRequiresType(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Devices = []DeviceConfig{{Name: "ahu", DeviceID: 100}}
	cfg.Points = []PointConfig{{Name: "setpoint", Device: "ahu", Object: "analog-value:1", Writable: true}}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected validation error")
	}
}
