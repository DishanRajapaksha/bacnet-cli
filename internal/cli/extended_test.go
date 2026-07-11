package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DishanRajapaksha/bacnet-cli/internal/bacnetclient"
)

type pointClient struct{}

func (pointClient) Close() error { return nil }
func (pointClient) Discover(bacnetclient.DiscoveryOptions) ([]bacnetclient.Device, error) {
	return nil, nil
}
func (pointClient) ReadProperty(target bacnetclient.Target, object bacnetclient.ObjectIdentifier, property bacnetclient.PropertyIdentifier, arrayIndex uint32) (bacnetclient.PropertyValue, error) {
	return bacnetclient.PropertyValue{
		Timestamp: time.Unix(0, 0).UTC(), DeviceID: target.DeviceID,
		Object: object, Property: property, ArrayIndex: arrayIndex,
		Value: float32(21.5), ValueType: "float32",
	}, nil
}
func (pointClient) WriteProperty(bacnetclient.WriteRequest) error { return nil }
func (pointClient) Objects(bacnetclient.Target) ([]bacnetclient.Object, error) {
	return nil, nil
}
func (pointClient) Identify(target bacnetclient.Target) (bacnetclient.DeviceIdentity, error) {
	property, _ := bacnetclient.ParsePropertyIdentifier("object-name")
	return bacnetclient.DeviceIdentity{
		Timestamp: time.Unix(0, 0).UTC(), DeviceID: target.DeviceID, Address: target.Address,
		Fields: []bacnetclient.IdentityField{{Property: property, Value: "AHU-1", ValueType: "string"}},
	}, nil
}
func (pointClient) Routers() ([]string, error) { return nil, nil }

func writeTestConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	contents := `connection:
  port: 47808
  subnet_cidr: 24
  timeout: 5s
output:
  format: table
devices:
  - name: ahu
    device_id: 100
    address: 192.0.2.10
points:
  - name: temperature
    device: ahu
    object: analog-input:1
    property: present-value
    unit: °C
  - name: setpoint
    device: ahu
    object: analog-value:1
    property: present-value
    type: float32
    unit: °C
    writable: true
    priority: 16
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestExtendedHelpIncludesNamedPoints(t *testing.T) {
	var out, errOut bytes.Buffer
	code := NewApp(&out, &errOut).RunV2([]string{"help"})
	if code != 0 || !strings.Contains(out.String(), "read-point") || !strings.Contains(out.String(), "identify") {
		t.Fatalf("code=%d output=%q errors=%q", code, out.String(), errOut.String())
	}
}

func TestGlobalFlagsBeforeReadPoint(t *testing.T) {
	path := writeTestConfig(t)
	factory := &fakeFactory{client: pointClient{}}
	var out, errOut bytes.Buffer
	code := NewAppWithFactory(&out, &errOut, factory).RunV2([]string{
		"--config", path, "--format", "json", "read-point", "temperature",
	})
	if code != 0 {
		t.Fatalf("code=%d errors=%q", code, errOut.String())
	}
	if factory.opened != 1 || !strings.Contains(out.String(), `"point": "temperature"`) || !strings.Contains(out.String(), `"unit": "°C"`) {
		t.Fatalf("opened=%d output=%q", factory.opened, out.String())
	}
}

func TestWritePointDryRunDoesNotOpenSocket(t *testing.T) {
	path := writeTestConfig(t)
	factory := &fakeFactory{client: pointClient{}}
	var out, errOut bytes.Buffer
	code := NewAppWithFactory(&out, &errOut, factory).RunV2([]string{
		"--config", path, "write-point", "setpoint", "--value", "22.0",
	})
	if code != 0 {
		t.Fatalf("code=%d errors=%q", code, errOut.String())
	}
	if factory.opened != 0 || !strings.Contains(out.String(), "setpoint") || !strings.Contains(out.String(), "true") {
		t.Fatalf("opened=%d output=%q", factory.opened, out.String())
	}
}

func TestIdentifyConfiguredDevice(t *testing.T) {
	path := writeTestConfig(t)
	factory := &fakeFactory{client: pointClient{}}
	var out, errOut bytes.Buffer
	code := NewAppWithFactory(&out, &errOut, factory).RunV2([]string{
		"--config", path, "identify", "ahu", "--format", "json",
	})
	if code != 0 {
		t.Fatalf("code=%d errors=%q", code, errOut.String())
	}
	if factory.opened != 1 || !strings.Contains(out.String(), "AHU-1") {
		t.Fatalf("opened=%d output=%q", factory.opened, out.String())
	}
}

func TestObjectTypesCatalog(t *testing.T) {
	var out, errOut bytes.Buffer
	code := NewAppWithFactory(&out, &errOut, &fakeFactory{}).RunV2([]string{"object-types", "--format", "json"})
	if code != 0 || !strings.Contains(out.String(), "analog-input") || !strings.Contains(out.String(), "ai") {
		t.Fatalf("code=%d output=%q errors=%q", code, out.String(), errOut.String())
	}
}
