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
