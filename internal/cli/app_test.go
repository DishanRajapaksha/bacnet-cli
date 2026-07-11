package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/DishanRajapaksha/bacnet-cli/internal/bacnetclient"
)

type fakeFactory struct {
	opened int
	client bacnetclient.Client
	err    error
}

func (f *fakeFactory) Open(bacnetclient.Options) (bacnetclient.Client, error) {
	f.opened++
	return f.client, f.err
}

type fakeClient struct{}

func (fakeClient) Close() error { return nil }
func (fakeClient) Discover(bacnetclient.DiscoveryOptions) ([]bacnetclient.Device, error) {
	return []bacnetclient.Device{{DeviceID: 100, Address: "192.0.2.10", Port: 47808}}, nil
}
func (fakeClient) ReadProperty(bacnetclient.Target, bacnetclient.ObjectIdentifier, bacnetclient.PropertyIdentifier, uint32) (bacnetclient.PropertyValue, error) {
	return bacnetclient.PropertyValue{}, errors.New("not implemented")
}
func (fakeClient) WriteProperty(bacnetclient.WriteRequest) error              { return nil }
func (fakeClient) Objects(bacnetclient.Target) ([]bacnetclient.Object, error) { return nil, nil }
func (fakeClient) Identify(bacnetclient.Target) (bacnetclient.DeviceIdentity, error) {
	return bacnetclient.DeviceIdentity{}, nil
}
func (fakeClient) Routers() ([]string, error) { return nil, nil }

func TestHelp(t *testing.T) {
	var out, errOut bytes.Buffer
	code := NewApp(&out, &errOut).Run([]string{"help"})
	if code != 0 || !strings.Contains(out.String(), "discover") {
		t.Fatalf("code=%d output=%q errors=%q", code, out.String(), errOut.String())
	}
}

func TestGlobalFlagsBeforeCommand(t *testing.T) {
	factory := &fakeFactory{client: fakeClient{}}
	var out, errOut bytes.Buffer
	code := NewAppWithFactory(&out, &errOut, factory).Run([]string{"--format", "json", "discover", "--low", "100", "--high", "100"})
	if code != 0 {
		t.Fatalf("code=%d errors=%q", code, errOut.String())
	}
	if factory.opened != 1 || !strings.Contains(out.String(), "device_id") {
		t.Fatalf("opened=%d output=%q", factory.opened, out.String())
	}
}

func TestWriteIsDryRunByDefault(t *testing.T) {
	factory := &fakeFactory{client: fakeClient{}}
	var out, errOut bytes.Buffer
	code := NewAppWithFactory(&out, &errOut, factory).Run([]string{
		"write", "--device-id", "100", "--object", "analog-output:1",
		"--property", "present-value", "--type", "float32", "--value", "21.5",
	})
	if code != 0 {
		t.Fatalf("code=%d errors=%q", code, errOut.String())
	}
	if factory.opened != 0 {
		t.Fatalf("dry run opened a socket %d times", factory.opened)
	}
	if !strings.Contains(out.String(), "true") {
		t.Fatalf("output does not show dry run: %q", out.String())
	}
}
