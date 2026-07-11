package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"strings"
	"syscall"
	"time"

	"github.com/DishanRajapaksha/bacnet-cli/internal/bacnetclient"
	"github.com/DishanRajapaksha/bacnet-cli/internal/devicemap"
	"github.com/DishanRajapaksha/bacnet-cli/internal/output"
)

func (a *App) devices(args []string) error {
	fs := a.newFlagSet("devices")
	common := addCommonFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return &usageError{message: "devices does not accept positional arguments"}
	}
	cfg, err := common.loadConfig()
	if err != nil {
		return err
	}
	return renderConfiguredDevices(a.out, cfg.Output.Format, devicemap.ListDevices(cfg.Devices))
}

func (a *App) points(args []string) error {
	fs := a.newFlagSet("points")
	common := addCommonFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return &usageError{message: "points does not accept positional arguments"}
	}
	cfg, err := common.loadConfig()
	if err != nil {
		return err
	}
	return renderConfiguredPoints(a.out, cfg.Output.Format, devicemap.ListPoints(cfg.Points))
}

func (a *App) readPoint(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("%w: point name is required", bacnetclient.ErrValidation)
	}
	if args[0] == "--help" || args[0] == "-h" {
		return flag.ErrHelp
	}
	name := args[0]
	fs := a.newFlagSet("read-point")
	common := addCommonFlags(fs)
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	cfg, err := common.loadConfig()
	if err != nil {
		return err
	}
	resolved, err := devicemap.ResolvePoint(cfg, name)
	if err != nil {
		return err
	}
	client, _, err := a.open(common)
	if err != nil {
		return err
	}
	defer client.Close()
	value, err := client.ReadProperty(resolved.Target, resolved.Object, resolved.Property, resolved.ArrayIndex)
	if err != nil {
		return err
	}
	return renderPointValue(a.out, cfg.Output.Format, devicemap.ApplyPoint(resolved.Point, value))
}

func (a *App) writePoint(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("%w: point name is required", bacnetclient.ErrValidation)
	}
	if args[0] == "--help" || args[0] == "-h" {
		return flag.ErrHelp
	}
	name := args[0]
	fs := a.newFlagSet("write-point")
	common := addCommonFlags(fs)
	rawValue := fs.String("value", "", "value to write")
	isNull := fs.Bool("null", false, "write BACnet null to relinquish the selected priority")
	dryRun := fs.Bool("dry-run", false, "print the write plan without sending")
	yes := fs.Bool("yes", false, "send the write request")
	priority := fs.Uint("priority", 0, "override BACnet command priority, 1 to 16")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *dryRun && *yes {
		return fmt.Errorf("%w: --dry-run and --yes cannot be used together", bacnetclient.ErrValidation)
	}
	if *priority > 16 {
		return fmt.Errorf("%w: --priority must be between 1 and 16", bacnetclient.ErrValidation)
	}
	cfg, err := common.loadConfig()
	if err != nil {
		return err
	}
	resolved, err := devicemap.ResolvePoint(cfg, name)
	if err != nil {
		return err
	}
	if !resolved.Point.Writable {
		return fmt.Errorf("%w: point %q is not marked writable", bacnetclient.ErrWriteRejected, name)
	}
	value, err := bacnetclient.ParseWriteValue(resolved.Point.Type, *rawValue, *isNull)
	if err != nil {
		return err
	}
	writePriority := resolved.Priority
	if *priority != 0 {
		writePriority = uint8(*priority)
	}
	valueType := "<nil>"
	if value != nil {
		valueType = reflect.TypeOf(value).String()
	}
	plan := bacnetclient.WritePlan{
		Point:      resolved.Point.Name,
		Unit:       resolved.Point.Unit,
		DeviceID:   resolved.Target.DeviceID,
		Address:    resolved.Target.Address,
		Object:     resolved.Object,
		Property:   resolved.Property,
		ArrayIndex: resolved.ArrayIndex,
		Priority:   writePriority,
		Value:      value,
		ValueType:  valueType,
		DryRun:     !*yes,
	}
	if !*yes {
		return renderPointWritePlan(a.out, cfg.Output.Format, plan)
	}
	client, _, err := a.open(common)
	if err != nil {
		return err
	}
	defer client.Close()
	if err := client.WriteProperty(bacnetclient.WriteRequest{
		Target: resolved.Target, Object: resolved.Object, Property: resolved.Property,
		ArrayIndex: resolved.ArrayIndex, Priority: writePriority, Value: value,
	}); err != nil {
		return err
	}
	plan.DryRun = false
	return renderPointWritePlan(a.out, cfg.Output.Format, plan)
}

func (a *App) watchPoint(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("%w: point name is required", bacnetclient.ErrValidation)
	}
	if args[0] == "--help" || args[0] == "-h" {
		return flag.ErrHelp
	}
	name := args[0]
	fs := a.newFlagSet("watch-point")
	common := addCommonFlags(fs)
	interval := fs.Duration("interval", time.Second, "poll interval")
	duration := fs.Duration("duration", 0, "stop after this duration; zero runs until interrupted")
	count := fs.Int("count", 0, "number of reads; zero runs until interrupted")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *interval <= 0 || *duration < 0 || *count < 0 {
		return fmt.Errorf("%w: interval must be positive and duration/count must not be negative", bacnetclient.ErrValidation)
	}
	cfg, err := common.loadConfig()
	if err != nil {
		return err
	}
	resolved, err := devicemap.ResolvePoint(cfg, name)
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
	for n := 0; *count == 0 || n < *count; n++ {
		value, readErr := client.ReadProperty(resolved.Target, resolved.Object, resolved.Property, resolved.ArrayIndex)
		if readErr != nil {
			return readErr
		}
		value = devicemap.ApplyPoint(resolved.Point, value)
		if err := renderPointStream(a.out, format, value, csvHeader); err != nil {
			return err
		}
		csvHeader = false
		if *count > 0 && n+1 >= *count {
			return nil
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
	return nil
}

func (a *App) identify(args []string) error {
	name := ""
	flagArgs := args
	if len(args) > 0 && args[0] != "--help" && args[0] != "-h" && !strings.HasPrefix(args[0], "-") {
		name = args[0]
		flagArgs = args[1:]
	}
	fs := a.newFlagSet("identify")
	common := addCommonFlags(fs)
	targetFlags := addTargetFlags(fs)
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return &usageError{message: "identify accepts at most one configured device name"}
	}
	cfg, err := common.loadConfig()
	if err != nil {
		return err
	}
	var target bacnetclient.Target
	if name != "" {
		device, findErr := devicemap.FindDevice(cfg.Devices, name)
		if findErr != nil {
			return findErr
		}
		target = devicemap.ResolveDevice(device)
	} else {
		target, err = targetFlags.target()
		if err != nil {
			return err
		}
	}
	client, _, err := a.open(common)
	if err != nil {
		return err
	}
	defer client.Close()
	identity, err := client.Identify(target)
	if err != nil {
		return err
	}
	return renderIdentity(a.out, cfg.Output.Format, identity)
}

func (a *App) objectTypes(args []string) error {
	fs := a.newFlagSet("object-types")
	format := fs.String("format", output.FormatTable, "output format: table, text, json, or csv")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return &usageError{message: "object-types does not accept positional arguments"}
	}
	return renderCatalog(a.out, *format, bacnetclient.ObjectTypeCatalog())
}

func (a *App) properties(args []string) error {
	fs := a.newFlagSet("properties")
	format := fs.String("format", output.FormatTable, "output format: table, text, json, or csv")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return &usageError{message: "properties does not accept positional arguments"}
	}
	return renderCatalog(a.out, *format, bacnetclient.PropertyCatalog())
}
