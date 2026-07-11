package devicemap

import (
	"testing"

	"github.com/DishanRajapaksha/bacnet-cli/internal/config"
)

func TestResolvePointDefaults(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Devices = []config.DeviceConfig{{Name: "ahu", DeviceID: 100}}
	cfg.Points = []config.PointConfig{{
		Name: "temperature", Device: "ahu", Object: "analog-input:1",
	}}
	resolved, err := ResolvePoint(cfg, "temperature")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Target.Port != 47808 || resolved.Target.MaxAPDU != 1476 || resolved.Target.Segmentation != 3 {
		t.Fatalf("unexpected target defaults: %#v", resolved.Target)
	}
	if resolved.Property.ID != 85 || resolved.ArrayIndex != ^uint32(0) || resolved.Priority != 16 {
		t.Fatalf("unexpected point defaults: %#v", resolved)
	}
}

func TestResolvePointUsesOverrides(t *testing.T) {
	index := uint32(2)
	segmentation := uint32(1)
	mstp := 7
	cfg := config.DefaultConfig()
	cfg.Devices = []config.DeviceConfig{{
		Name: "vav", DeviceID: 200, Address: "192.0.2.20", Port: 47809,
		Network: 10, MSTPMAC: &mstp, MaxAPDU: 480, Segmentation: &segmentation,
	}}
	cfg.Points = []config.PointConfig{{
		Name: "setpoint", Device: "vav", Object: "analog-value:2",
		Property: "present-value", ArrayIndex: &index, Type: "float32", Writable: true, Priority: 8,
	}}
	resolved, err := ResolvePoint(cfg, "setpoint")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Target.Port != 47809 || resolved.Target.NetworkNumber != 10 || resolved.Target.MSTPMAC == nil || *resolved.Target.MSTPMAC != 7 {
		t.Fatalf("unexpected target: %#v", resolved.Target)
	}
	if resolved.ArrayIndex != 2 || resolved.Priority != 8 {
		t.Fatalf("unexpected point: %#v", resolved)
	}
}
