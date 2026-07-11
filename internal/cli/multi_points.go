package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/DishanRajapaksha/bacnet-cli/internal/bacnetclient"
	"github.com/DishanRajapaksha/bacnet-cli/internal/config"
	"github.com/DishanRajapaksha/bacnet-cli/internal/devicemap"
	"github.com/DishanRajapaksha/bacnet-cli/internal/output"
)

type stringListFlag []string

func (f *stringListFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *stringListFlag) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("point name must not be empty")
	}
	*f = append(*f, value)
	return nil
}

type pointSample struct {
	Timestamp time.Time                       `json:"timestamp"`
	Cycle     int                             `json:"cycle,omitempty"`
	Point     string                          `json:"point"`
	Device    string                          `json:"device"`
	DeviceID  int                             `json:"device_id"`
	Object    bacnetclient.ObjectIdentifier   `json:"object"`
	Property  bacnetclient.PropertyIdentifier `json:"property"`
	Value     any                             `json:"value"`
	ValueType string                          `json:"value_type,omitempty"`
	Unit      string                          `json:"unit,omitempty"`
	Error     string                          `json:"error,omitempty"`
}

func (a *App) readPoints(args []string) error {
	fs := a.newFlagSet("read-points")
	common := addCommonFlags(fs)
	var selected stringListFlag
	fs.Var(&selected, "point", "configured point name; repeat for multiple points")
	device := fs.String("device", "", "read every configured point belonging to this device")
	failFast := fs.Bool("fail-fast", false, "stop after the first failed point read")
	if err := fs.Parse(args); err != nil {
		return err
	}
	selected = append(selected, fs.Args()...)

	cfg, err := common.loadConfig()
	if err != nil {
		return err
	}
	points, err := resolveSelectedPoints(cfg, selected, *device)
	if err != nil {
		return err
	}
	client, _, err := a.open(common)
	if err != nil {
		return err
	}
	defer client.Close()

	samples := make([]pointSample, 0, len(points))
	failures := 0
	for _, point := range points {
		sample := readPointSample(client, point, 0)
		samples = append(samples, sample)
		if sample.Error != "" {
			failures++
			if *failFast {
				break
			}
		}
	}
	if err := renderPointSamples(a.out, cfg.Output.Format, samples); err != nil {
		return err
	}
	if failures > 0 {
		return fmt.Errorf("%w: %d of %d point reads failed", bacnetclient.ErrRequest, failures, len(samples))
	}
	return nil
}

func (a *App) watchPoints(args []string) error {
	fs := a.newFlagSet("watch-points")
	common := addCommonFlags(fs)
	var selected stringListFlag
	fs.Var(&selected, "point", "configured point name; repeat for multiple points")
	device := fs.String("device", "", "watch every configured point belonging to this device")
	interval := fs.Duration("interval", time.Second, "poll interval between complete point cycles")
	duration := fs.Duration("duration", 0, "stop after this duration; zero runs until interrupted")
	count := fs.Int("count", 0, "number of complete polling cycles; zero runs until interrupted")
	failFast := fs.Bool("fail-fast", false, "stop after the first failed point read")
	if err := fs.Parse(args); err != nil {
		return err
	}
	selected = append(selected, fs.Args()...)
	if *interval <= 0 || *duration < 0 || *count < 0 {
		return fmt.Errorf("%w: interval must be positive and duration/count must not be negative", bacnetclient.ErrValidation)
	}

	cfg, err := common.loadConfig()
	if err != nil {
		return err
	}
	points, err := resolveSelectedPoints(cfg, selected, *device)
	if err != nil {
		return err
	}
	format := output.NormaliseFormat(cfg.Output.Format)
	if format == output.FormatTable || format == output.FormatJSON {
		format = output.FormatText
	}
	if err := output.ValidateStreamFormat(format); err != nil {
		return err
	}
	client, _, err := a.open(common)
	if err != nil {
		return err
	}
	defer client.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if *duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *duration)
		defer cancel()
	}
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	csvHeader := true
	failures := 0
	for cycle := 1; *count == 0 || cycle <= *count; cycle++ {
		for _, point := range points {
			sample := readPointSample(client, point, cycle)
			if err := renderPointSample(a.out, format, sample, csvHeader); err != nil {
				return err
			}
			csvHeader = false
			if sample.Error != "" {
				failures++
				if *failFast {
					return fmt.Errorf("%w: point %q read failed: %s", bacnetclient.ErrRequest, sample.Point, sample.Error)
				}
			}
		}
		if *count > 0 && cycle >= *count {
			break
		}
		select {
		case <-ctx.Done():
			cycle = *count
		case <-ticker.C:
		}
		if ctx.Err() != nil {
			break
		}
	}
	if failures > 0 {
		return fmt.Errorf("%w: %d point reads failed while watching", bacnetclient.ErrRequest, failures)
	}
	return nil
}

func resolveSelectedPoints(cfg config.Config, names []string, device string) ([]devicemap.ResolvedPoint, error) {
	device = strings.TrimSpace(device)
	if device != "" {
		if _, err := devicemap.FindDevice(cfg.Devices, device); err != nil {
			return nil, err
		}
	}

	orderedNames := make([]string, 0, len(cfg.Points))
	if len(names) > 0 {
		orderedNames = append(orderedNames, names...)
	} else {
		for _, point := range devicemap.ListPoints(cfg.Points) {
			if device == "" || point.Device == device {
				orderedNames = append(orderedNames, point.Name)
			}
		}
	}

	seen := make(map[string]struct{}, len(orderedNames))
	resolved := make([]devicemap.ResolvedPoint, 0, len(orderedNames))
	for _, name := range orderedNames {
		name = strings.TrimSpace(name)
		if name == "" {
			return nil, fmt.Errorf("%w: point name must not be empty", bacnetclient.ErrValidation)
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		point, err := devicemap.ResolvePoint(cfg, name)
		if err != nil {
			return nil, err
		}
		if device != "" && point.Point.Device != device {
			return nil, fmt.Errorf("%w: point %q belongs to device %q, not %q", config.ErrConfig, name, point.Point.Device, device)
		}
		resolved = append(resolved, point)
	}
	if len(resolved) == 0 {
		return nil, fmt.Errorf("%w: no configured points matched the selection", config.ErrConfig)
	}
	return resolved, nil
}

func readPointSample(client bacnetclient.Client, point devicemap.ResolvedPoint, cycle int) pointSample {
	sample := pointSample{
		Timestamp: time.Now().UTC(),
		Cycle:     cycle,
		Point:     point.Point.Name,
		Device:    point.Point.Device,
		DeviceID:  point.Target.DeviceID,
		Object:    point.Object,
		Property:  point.Property,
		Unit:      point.Point.Unit,
	}
	value, err := client.ReadProperty(point.Target, point.Object, point.Property, point.ArrayIndex)
	if err != nil {
		sample.Error = err.Error()
		return sample
	}
	value = devicemap.ApplyPoint(point.Point, value)
	if !value.Timestamp.IsZero() {
		sample.Timestamp = value.Timestamp
	}
	sample.Value = value.Value
	sample.ValueType = value.ValueType
	return sample
}

func renderPointSamples(w io.Writer, format string, samples []pointSample) error {
	format = output.NormaliseFormat(format)
	if err := output.ValidateSnapshotFormat(format); err != nil {
		return err
	}
	if format == output.FormatJSON {
		return output.WriteJSON(w, samples)
	}
	rows := make([][]string, 0, len(samples))
	for _, sample := range samples {
		rows = append(rows, pointSampleRow(sample, false))
	}
	return renderRows(w, format, pointSampleHeaders(false), rows)
}

func renderPointSample(w io.Writer, format string, sample pointSample, csvHeader bool) error {
	format = output.NormaliseFormat(format)
	if err := output.ValidateStreamFormat(format); err != nil {
		return err
	}
	if format == output.FormatJSONL {
		return output.WriteJSONLine(w, sample)
	}
	row := pointSampleRow(sample, true)
	if format == output.FormatCSV {
		headers := []string(nil)
		if csvHeader {
			headers = pointSampleHeaders(true)
		}
		return output.WriteCSV(w, headers, [][]string{row})
	}
	return output.WriteText(w, strings.Join(row, " "))
}

func pointSampleHeaders(stream bool) []string {
	headers := []string{"TIMESTAMP"}
	if stream {
		headers = append(headers, "CYCLE")
	}
	return append(headers, "POINT", "DEVICE", "DEVICE_ID", "OBJECT", "PROPERTY", "VALUE", "UNIT", "TYPE", "ERROR")
}

func pointSampleRow(sample pointSample, stream bool) []string {
	row := []string{sample.Timestamp.Format("2006-01-02T15:04:05.000Z07:00")}
	if stream {
		row = append(row, strconv.Itoa(sample.Cycle))
	}
	return append(row,
		sample.Point,
		sample.Device,
		strconv.Itoa(sample.DeviceID),
		fmt.Sprintf("%s:%d", sample.Object.TypeName, sample.Object.Instance),
		sample.Property.Name,
		formatValue(sample.Value),
		sample.Unit,
		sample.ValueType,
		sample.Error,
	)
}

var _ flag.Value = (*stringListFlag)(nil)
