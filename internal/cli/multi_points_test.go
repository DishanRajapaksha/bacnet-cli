package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/DishanRajapaksha/bacnet-cli/internal/bacnetclient"
)

type partiallyFailingPointClient struct {
	pointClient
}

func (partiallyFailingPointClient) ReadProperty(target bacnetclient.Target, object bacnetclient.ObjectIdentifier, property bacnetclient.PropertyIdentifier, arrayIndex uint32) (bacnetclient.PropertyValue, error) {
	if object.TypeName == "analog-value" {
		return bacnetclient.PropertyValue{}, errors.New("simulated BACnet reject")
	}
	return pointClient{}.ReadProperty(target, object, property, arrayIndex)
}

func TestReadPointsUsesOneSessionAndReturnsAllConfiguredPoints(t *testing.T) {
	path := writeTestConfig(t)
	factory := &fakeFactory{client: pointClient{}}
	var out, errOut bytes.Buffer
	code := NewAppWithFactory(&out, &errOut, factory).RunV2([]string{
		"--config", path, "--format", "json", "read-points",
	})
	if code != 0 {
		t.Fatalf("code=%d errors=%q", code, errOut.String())
	}
	if factory.opened != 1 {
		t.Fatalf("expected one BACnet session, got %d", factory.opened)
	}
	if !strings.Contains(out.String(), `"point": "setpoint"`) || !strings.Contains(out.String(), `"point": "temperature"`) {
		t.Fatalf("missing configured points: %q", out.String())
	}
	if strings.Index(out.String(), `"point": "setpoint"`) > strings.Index(out.String(), `"point": "temperature"`) {
		t.Fatalf("points are not rendered in deterministic name order: %q", out.String())
	}
}

func TestReadPointsSupportsRepeatedSelectors(t *testing.T) {
	path := writeTestConfig(t)
	factory := &fakeFactory{client: pointClient{}}
	var out, errOut bytes.Buffer
	code := NewAppWithFactory(&out, &errOut, factory).RunV2([]string{
		"--config", path, "--format", "json", "read-points",
		"--point", "temperature", "--point", "temperature",
	})
	if code != 0 {
		t.Fatalf("code=%d errors=%q", code, errOut.String())
	}
	if strings.Count(out.String(), `"point": "temperature"`) != 1 {
		t.Fatalf("duplicate point selector was not collapsed: %q", out.String())
	}
	if strings.Contains(out.String(), `"point": "setpoint"`) {
		t.Fatalf("unselected point was rendered: %q", out.String())
	}
}

func TestReadPointsEmitsPartialResultsAndReturnsProtocolExitCode(t *testing.T) {
	path := writeTestConfig(t)
	factory := &fakeFactory{client: partiallyFailingPointClient{}}
	var out, errOut bytes.Buffer
	code := NewAppWithFactory(&out, &errOut, factory).RunV2([]string{
		"--config", path, "--format", "json", "read-points",
	})
	if code != exitRequestError {
		t.Fatalf("code=%d output=%q errors=%q", code, out.String(), errOut.String())
	}
	if !strings.Contains(out.String(), "simulated BACnet reject") || !strings.Contains(out.String(), `"point": "temperature"`) {
		t.Fatalf("partial output was not preserved: %q", out.String())
	}
}

func TestReadPointsRejectsUnknownPointBeforeOpeningSocket(t *testing.T) {
	path := writeTestConfig(t)
	factory := &fakeFactory{client: pointClient{}}
	var out, errOut bytes.Buffer
	code := NewAppWithFactory(&out, &errOut, factory).RunV2([]string{
		"--config", path, "read-points", "--point", "missing",
	})
	if code != exitConfigError {
		t.Fatalf("code=%d output=%q errors=%q", code, out.String(), errOut.String())
	}
	if factory.opened != 0 {
		t.Fatalf("invalid selection opened a BACnet socket %d times", factory.opened)
	}
}

func TestWatchPointsWritesOneCSVHeaderAcrossCycles(t *testing.T) {
	path := writeTestConfig(t)
	factory := &fakeFactory{client: pointClient{}}
	var out, errOut bytes.Buffer
	code := NewAppWithFactory(&out, &errOut, factory).RunV2([]string{
		"--config", path, "--format", "csv", "watch-points",
		"--count", "2", "--interval", "1ms",
	})
	if code != 0 {
		t.Fatalf("code=%d errors=%q", code, errOut.String())
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 5 {
		t.Fatalf("expected one header and four samples, got %d lines: %q", len(lines), out.String())
	}
	if strings.Count(out.String(), "TIMESTAMP,CYCLE,POINT") != 1 {
		t.Fatalf("CSV header was repeated: %q", out.String())
	}
}
