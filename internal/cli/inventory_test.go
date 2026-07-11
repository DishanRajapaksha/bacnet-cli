package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DishanRajapaksha/bacnet-cli/internal/bacnetclient"
	"github.com/DishanRajapaksha/bacnet-cli/internal/config"
	"gopkg.in/yaml.v3"
)

type inventoryTestClient struct {
	pointClient
	devices       []bacnetclient.Device
	objectNames   map[int]string
	identityFails map[int]bool
	identifyCalls int
}

func (c *inventoryTestClient) Discover(bacnetclient.DiscoveryOptions) ([]bacnetclient.Device, error) {
	return append([]bacnetclient.Device(nil), c.devices...), nil
}

func (c *inventoryTestClient) Identify(target bacnetclient.Target) (bacnetclient.DeviceIdentity, error) {
	c.identifyCalls++
	if c.identityFails[target.DeviceID] {
		return bacnetclient.DeviceIdentity{}, errors.New("simulated identity timeout")
	}
	objectName, _ := bacnetclient.ParsePropertyIdentifier("object-name")
	vendorName, _ := bacnetclient.ParsePropertyIdentifier("vendor-name")
	modelName, _ := bacnetclient.ParsePropertyIdentifier("model-name")
	return bacnetclient.DeviceIdentity{
		Timestamp: time.Unix(0, 0).UTC(),
		DeviceID:  target.DeviceID,
		Address:   target.Address,
		Fields: []bacnetclient.IdentityField{
			{Property: objectName, Value: c.objectNames[target.DeviceID], ValueType: "string"},
			{Property: vendorName, Value: "Example Controls", ValueType: "string"},
			{Property: modelName, Value: "EC-100", ValueType: "string"},
		},
	}, nil
}

func testDiscoveredDevices() []bacnetclient.Device {
	return []bacnetclient.Device{
		{DeviceID: 101, Address: "192.0.2.11", Port: 47808, MaxAPDU: 1476, Segmentation: 3, VendorID: 77},
		{DeviceID: 100, Address: "192.0.2.10", Port: 47808, MaxAPDU: 480, Segmentation: 3, VendorID: 77},
	}
}

func TestInventoryUsesOneSessionAndSortsDevices(t *testing.T) {
	path := writeTestConfig(t)
	client := &inventoryTestClient{
		devices:     testDiscoveredDevices(),
		objectNames: map[int]string{100: "AHU East", 101: "AHU West"},
	}
	factory := &fakeFactory{client: client}
	var out, errOut bytes.Buffer
	code := NewAppWithFactory(&out, &errOut, factory).RunV2([]string{
		"--config", path, "--format", "json", "inventory",
	})
	if code != 0 {
		t.Fatalf("code=%d errors=%q", code, errOut.String())
	}
	if factory.opened != 1 || client.identifyCalls != 2 {
		t.Fatalf("opened=%d identify_calls=%d", factory.opened, client.identifyCalls)
	}
	if strings.Index(out.String(), `"device_id": 100`) > strings.Index(out.String(), `"device_id": 101`) {
		t.Fatalf("inventory is not sorted by device id: %q", out.String())
	}
	if !strings.Contains(out.String(), `"object_name": "AHU East"`) || !strings.Contains(out.String(), `"vendor_name": "Example Controls"`) {
		t.Fatalf("identity metadata is missing: %q", out.String())
	}
}

func TestInventoryPreservesPartialResultsAndReturnsRequestExitCode(t *testing.T) {
	path := writeTestConfig(t)
	client := &inventoryTestClient{
		devices:       testDiscoveredDevices(),
		objectNames:   map[int]string{100: "AHU East"},
		identityFails: map[int]bool{101: true},
	}
	factory := &fakeFactory{client: client}
	var out, errOut bytes.Buffer
	code := NewAppWithFactory(&out, &errOut, factory).RunV2([]string{
		"--config", path, "--format", "json", "inventory",
	})
	if code != exitRequestError {
		t.Fatalf("code=%d output=%q errors=%q", code, out.String(), errOut.String())
	}
	if !strings.Contains(out.String(), `"object_name": "AHU East"`) || !strings.Contains(out.String(), "simulated identity timeout") {
		t.Fatalf("partial inventory was not preserved: %q", out.String())
	}
}

func TestGenerateConfigProducesSafeUniqueDeviceNames(t *testing.T) {
	path := writeTestConfig(t)
	client := &inventoryTestClient{
		devices:     testDiscoveredDevices(),
		objectNames: map[int]string{100: "AHU East", 101: "AHU East"},
	}
	factory := &fakeFactory{client: client}
	var out, errOut bytes.Buffer
	code := NewAppWithFactory(&out, &errOut, factory).RunV2([]string{
		"--config", path, "generate-config",
	})
	if code != 0 {
		t.Fatalf("code=%d errors=%q", code, errOut.String())
	}
	var generated config.FileConfig
	if err := yaml.Unmarshal(out.Bytes(), &generated); err != nil {
		t.Fatalf("generated YAML is invalid: %v\n%s", err, out.String())
	}
	if len(generated.Devices) != 2 {
		t.Fatalf("expected two generated devices, got %#v", generated.Devices)
	}
	if generated.Devices[0].Name != "ahu_east" || generated.Devices[1].Name != "ahu_east_101" {
		t.Fatalf("unexpected generated names: %#v", generated.Devices)
	}
	if generated.Devices[0].DeviceID != 100 || generated.Devices[1].DeviceID != 101 {
		t.Fatalf("generated devices are not sorted: %#v", generated.Devices)
	}
}

func TestGenerateConfigRefusesOverwriteBeforeOpeningSocket(t *testing.T) {
	path := writeTestConfig(t)
	outputPath := filepath.Join(t.TempDir(), "discovered.yaml")
	if err := os.WriteFile(outputPath, []byte("existing: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	client := &inventoryTestClient{devices: testDiscoveredDevices()}
	factory := &fakeFactory{client: client}
	var out, errOut bytes.Buffer
	code := NewAppWithFactory(&out, &errOut, factory).RunV2([]string{
		"--config", path, "generate-config", "--output", outputPath,
	})
	if code != exitConfigError {
		t.Fatalf("code=%d output=%q errors=%q", code, out.String(), errOut.String())
	}
	if factory.opened != 0 {
		t.Fatalf("overwrite refusal opened a BACnet socket %d times", factory.opened)
	}
	contents, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "existing: true\n" {
		t.Fatalf("existing file was modified: %q", contents)
	}
}

func TestGenerateConfigCanSkipIdentityReads(t *testing.T) {
	path := writeTestConfig(t)
	client := &inventoryTestClient{devices: testDiscoveredDevices()}
	factory := &fakeFactory{client: client}
	var out, errOut bytes.Buffer
	code := NewAppWithFactory(&out, &errOut, factory).RunV2([]string{
		"--config", path, "generate-config", "--identify=false",
	})
	if code != 0 {
		t.Fatalf("code=%d errors=%q", code, errOut.String())
	}
	if client.identifyCalls != 0 {
		t.Fatalf("identity reads were not disabled: %d", client.identifyCalls)
	}
	if !strings.Contains(out.String(), "name: device_100") || !strings.Contains(out.String(), "name: device_101") {
		t.Fatalf("fallback names are missing: %q", out.String())
	}
}
