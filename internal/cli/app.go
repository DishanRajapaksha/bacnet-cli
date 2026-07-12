package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"reflect"
	"strings"
	"syscall"
	"time"

	"github.com/DishanRajapaksha/bacnet-cli/internal/bacnetclient"
	"github.com/DishanRajapaksha/bacnet-cli/internal/config"
	"github.com/DishanRajapaksha/bacnet-cli/internal/output"
	"github.com/DishanRajapaksha/industrial-cli-kit/exitcode"
)

const (
	appName           = "bacnet-cli"
	exitSuccess       = int(exitcode.Success)
	exitGeneralError  = int(exitcode.General)
	exitConfigError   = int(exitcode.Config)
	exitConnection    = int(exitcode.Connection)
	exitRequestError  = int(exitcode.Request)
	exitWriteRejected = int(exitcode.Rejected)
	exitTimeout       = int(exitcode.Timeout)
	exitOutputError   = int(exitcode.Output)
)

type App struct {
	out     io.Writer
	err     io.Writer
	factory bacnetclient.Factory
}

type usageError struct{ message string }

func (e *usageError) Error() string { return e.message }

func NewApp(out io.Writer, err io.Writer) *App {
	return &App{out: out, err: err, factory: bacnetclient.NubeFactory{}}
}

func NewAppWithFactory(out io.Writer, err io.Writer, factory bacnetclient.Factory) *App {
	return &App{out: out, err: err, factory: factory}
}

func Main() {
	code := NewApp(os.Stdout, os.Stderr).Run(os.Args[1:])
	if code != 0 {
		os.Exit(code)
	}
}

func (a *App) Run(args []string) int {
	normalised, err := normaliseGlobalFlags(args)
	if err != nil {
		fmt.Fprintln(a.err, err)
		return exitConfigError
	}
	args = normalised
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		a.printUsage()
		return exitSuccess
	}
	if args[0] == "version" || args[0] == "--version" || args[0] == "-v" {
		fmt.Fprintf(a.out, "%s development\n", appName)
		return exitSuccess
	}

	switch args[0] {
	case "init-config":
		err = a.initConfig(args[1:])
	case "validate-config":
		err = a.validateConfig(args[1:])
	case "test-connection", "status":
		err = a.testConnection(args[1:])
	case "discover":
		err = a.discover(args[1:])
	case "read":
		err = a.read(args[1:])
	case "objects":
		err = a.objects(args[1:])
	case "write":
		err = a.write(args[1:])
	case "watch":
		err = a.watch(args[1:])
	case "routers":
		err = a.routers(args[1:])
	case "completions":
		err = a.completions(args[1:])
	default:
		a.printUsage()
		fmt.Fprintf(a.err, "unknown command %q\n", args[0])
		return exitGeneralError
	}
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitSuccess
		}
		fmt.Fprintln(a.err, err)
		return mapExitCode(err)
	}
	return exitSuccess
}

func mapExitCode(err error) int {
	var usage *usageError
	switch {
	case err == nil, errors.Is(err, flag.ErrHelp):
		return exitSuccess
	case errors.As(err, &usage), errors.Is(err, config.ErrConfig), errors.Is(err, bacnetclient.ErrValidation):
		return exitConfigError
	case errors.Is(err, context.DeadlineExceeded), strings.Contains(strings.ToLower(err.Error()), "timeout"):
		return exitTimeout
	case errors.Is(err, output.ErrOutput):
		return exitOutputError
	case errors.Is(err, bacnetclient.ErrConnection):
		return exitConnection
	case errors.Is(err, bacnetclient.ErrRequest):
		return exitRequestError
	case strings.Contains(err.Error(), "flag provided but not defined"), strings.Contains(err.Error(), "invalid value"):
		return exitConfigError
	default:
		return exitGeneralError
	}
}

func (a *App) printUsage() {
	fmt.Fprintln(a.out, `bacnet-cli is a script-friendly BACnet/IP command-line client.

Usage:
  bacnet-cli [global flags] <command> [flags]
  bacnet-cli init-config
  bacnet-cli validate-config --profile local
  bacnet-cli discover --low 0 --high 4194303
  bacnet-cli test-connection --device-id 1234
  bacnet-cli read --device-id 1234 --object analog-input:1 --property present-value
  bacnet-cli objects --device-id 1234
  bacnet-cli write --device-id 1234 --object analog-output:1 --property present-value --type float32 --value 21.5
  bacnet-cli write --device-id 1234 --object analog-output:1 --property present-value --type float32 --value 21.5 --yes
  bacnet-cli watch --device-id 1234 --object analog-input:1 --property present-value --interval 2s --format jsonl
  bacnet-cli routers
  bacnet-cli completions zsh

Commands:
  init-config       Write a starter YAML config file
  validate-config  Validate local config without opening a socket
  test-connection  Open BACnet/IP and require a device response
  status           Alias for test-connection
  discover         Send Who-Is and list I-Am responses
  read             Read one property from one BACnet object
  objects          Read a device object list with names and descriptions
  write            Write one property; dry-run unless --yes is supplied
  watch            Poll one property repeatedly
  routers          Send Who-Is-Router-To-Network
  completions      Generate Bash or Zsh completion scripts
  version          Print version information

Global flags:
  --config       YAML config file, defaults to config.yaml
  --profile      Config profile name
  --interface    Local interface, for example en0 or eth0
  --local-ip     Local IPv4 address; alternative to --interface
  --port         Local BACnet/IP UDP port, default 47808
  --subnet-cidr  Local IPv4 subnet prefix length
  --timeout      Request timeout
  --format       snapshots: table, text, json, csv; streams: text, jsonl, csv
  --verbose      Print high-level connection decisions
  --debug        Enable lower-level debug logging

Target flags:
  --device-id       BACnet device instance; required for read/write/objects/watch
  --device-address  Device IPv4 address; omitted to discover by device id
  --device-port     Device BACnet/IP UDP port
  --network         BACnet network number
  --mstp-mac        Routed MS/TP MAC address
  --max-apdu        Device maximum APDU length
  --segmentation    Device segmentation mode`)
}

func (a *App) newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(a.err)
	return fs
}

func (a *App) open(common *commonFlagValues) (bacnetclient.Client, config.Config, error) {
	cfg, err := common.loadConfig()
	if err != nil {
		return nil, cfg, err
	}
	if common.verbose {
		iface := cfg.Connection.Interface
		if iface == "" {
			iface = "auto"
		}
		fmt.Fprintf(a.err, "BACnet/IP interface=%s local_ip=%s port=%d subnet=/%d\n", iface, cfg.Connection.LocalIP, cfg.Connection.Port, cfg.Connection.SubnetCIDR)
	}
	client, err := a.factory.Open(bacnetclient.Options{
		Interface: cfg.Connection.Interface, LocalIP: cfg.Connection.LocalIP,
		Port: cfg.Connection.Port, SubnetCIDR: cfg.Connection.SubnetCIDR, Timeout: cfg.Connection.Timeout,
	})
	return client, cfg, err
}

func (a *App) initConfig(args []string) error {
	fs := a.newFlagSet("init-config")
	path := fs.String("output", config.DefaultConfigPath, "output path")
	force := fs.Bool("force", false, "overwrite an existing file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return &usageError{message: "init-config does not accept positional arguments"}
	}
	if !*force {
		if _, err := os.Stat(*path); err == nil {
			return fmt.Errorf("%w: %s already exists; use --force", config.ErrConfig, *path)
		}
	}
	data, err := config.StarterConfigYAML()
	if err != nil {
		return fmt.Errorf("%w: create starter config: %v", config.ErrConfig, err)
	}
	if err := os.WriteFile(*path, data, 0o600); err != nil {
		return fmt.Errorf("%w: write config: %v", config.ErrConfig, err)
	}
	fmt.Fprintln(a.out, *path)
	return nil
}

func (a *App) validateConfig(args []string) error {
	fs := a.newFlagSet("validate-config")
	common := addCommonFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return &usageError{message: "validate-config does not accept positional arguments"}
	}
	cfg, err := common.loadConfig()
	if err != nil {
		return err
	}
	fmt.Fprintf(a.out, "valid interface=%s local_ip=%s port=%d format=%s\n", cfg.Connection.Interface, cfg.Connection.LocalIP, cfg.Connection.Port, cfg.Output.Format)
	return nil
}

func (a *App) testConnection(args []string) error {
	fs := a.newFlagSet("test-connection")
	common := addCommonFlags(fs)
	deviceID := fs.Int("device-id", -1, "optional device instance to require")
	if err := fs.Parse(args); err != nil {
		return err
	}
	client, cfg, err := a.open(common)
	if err != nil {
		return err
	}
	defer client.Close()
	low, high := 0, 4194303
	if *deviceID >= 0 {
		low, high = *deviceID, *deviceID
	}
	devices, err := client.Discover(bacnetclient.DiscoveryOptions{Low: low, High: high, GlobalBroadcast: true})
	if err != nil {
		return err
	}
	if len(devices) == 0 {
		return fmt.Errorf("%w: no BACnet devices answered Who-Is", bacnetclient.ErrConnection)
	}
	result := map[string]any{"status": "ok", "devices": len(devices), "port": cfg.Connection.Port}
	if *deviceID >= 0 {
		result["device_id"] = *deviceID
	}
	if output.NormaliseFormat(cfg.Output.Format) == output.FormatJSON {
		return output.WriteJSON(a.out, result)
	}
	fmt.Fprintf(a.out, "ok devices=%d port=%d\n", len(devices), cfg.Connection.Port)
	return nil
}

func (a *App) discover(args []string) error {
	fs := a.newFlagSet("discover")
	common := addCommonFlags(fs)
	low := fs.Int("low", 0, "lowest device instance")
	high := fs.Int("high", 4194303, "highest device instance")
	network := fs.Uint("network", 0, "BACnet network number")
	global := fs.Bool("global-broadcast", true, "use a global broadcast")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *low < 0 || *high < *low || *high > 4194303 {
		return &usageError{message: "discovery range must satisfy 0 <= low <= high <= 4194303"}
	}
	client, cfg, err := a.open(common)
	if err != nil {
		return err
	}
	defer client.Close()
	devices, err := client.Discover(bacnetclient.DiscoveryOptions{Low: *low, High: *high, GlobalBroadcast: *global, NetworkNumber: uint16(*network)})
	if err != nil {
		return err
	}
	return renderDevices(a.out, cfg.Output.Format, devices)
}

func parseReadFlags(a *App, name string, args []string) (*commonFlagValues, bacnetclient.Target, bacnetclient.ObjectIdentifier, bacnetclient.PropertyIdentifier, uint32, error) {
	fs := a.newFlagSet(name)
	common := addCommonFlags(fs)
	targetFlags := addTargetFlags(fs)
	objectText := fs.String("object", "", "BACnet object as TYPE:INSTANCE")
	propertyText := fs.String("property", "present-value", "property name or numeric identifier")
	arrayIndex := fs.Uint("array-index", uint(^uint32(0)), "array index; defaults to BACnet ARRAY_ALL")
	if err := fs.Parse(args); err != nil {
		return nil, bacnetclient.Target{}, bacnetclient.ObjectIdentifier{}, bacnetclient.PropertyIdentifier{}, 0, err
	}
	target, err := targetFlags.target()
	if err != nil {
		return nil, target, bacnetclient.ObjectIdentifier{}, bacnetclient.PropertyIdentifier{}, 0, err
	}
	object, err := bacnetclient.ParseObjectIdentifier(*objectText)
	if err != nil {
		return nil, target, object, bacnetclient.PropertyIdentifier{}, 0, err
	}
	property, err := bacnetclient.ParsePropertyIdentifier(*propertyText)
	return common, target, object, property, uint32(*arrayIndex), err
}

func (a *App) read(args []string) error {
	common, target, object, property, arrayIndex, err := parseReadFlags(a, "read", args)
	if err != nil {
		return err
	}
	client, cfg, err := a.open(common)
	if err != nil {
		return err
	}
	defer client.Close()
	value, err := client.ReadProperty(target, object, property, arrayIndex)
	if err != nil {
		return err
	}
	return renderProperty(a.out, cfg.Output.Format, value)
}

func (a *App) objects(args []string) error {
	fs := a.newFlagSet("objects")
	common := addCommonFlags(fs)
	targetFlags := addTargetFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	target, err := targetFlags.target()
	if err != nil {
		return err
	}
	client, cfg, err := a.open(common)
	if err != nil {
		return err
	}
	defer client.Close()
	objects, err := client.Objects(target)
	if err != nil {
		return err
	}
	return renderObjects(a.out, cfg.Output.Format, objects)
}

func (a *App) write(args []string) error {
	fs := a.newFlagSet("write")
	common := addCommonFlags(fs)
	targetFlags := addTargetFlags(fs)
	objectText := fs.String("object", "", "BACnet object as TYPE:INSTANCE")
	propertyText := fs.String("property", "present-value", "property name or numeric identifier")
	arrayIndex := fs.Uint("array-index", uint(^uint32(0)), "array index; defaults to BACnet ARRAY_ALL")
	valueText := fs.String("value", "", "value to write")
	valueType := fs.String("type", "", "string, bool, uint, int, float32, or float64")
	priority := fs.Uint("priority", 16, "BACnet command priority, 1 to 16")
	isNull := fs.Bool("null", false, "write BACnet null, commonly used to relinquish a priority")
	yes := fs.Bool("yes", false, "perform the write; otherwise print a dry run")
	if err := fs.Parse(args); err != nil {
		return err
	}
	target, err := targetFlags.target()
	if err != nil {
		return err
	}
	object, err := bacnetclient.ParseObjectIdentifier(*objectText)
	if err != nil {
		return err
	}
	property, err := bacnetclient.ParsePropertyIdentifier(*propertyText)
	if err != nil {
		return err
	}
	if *priority < 1 || *priority > 16 {
		return &usageError{message: "--priority must be between 1 and 16"}
	}
	value, err := bacnetclient.ParseWriteValue(*valueType, *valueText, *isNull)
	if err != nil {
		return err
	}
	cfg, err := common.loadConfig()
	if err != nil {
		return err
	}
	valueTypeName := "<nil>"
	if value != nil {
		valueTypeName = reflect.TypeOf(value).String()
	}
	plan := bacnetclient.WritePlan{
		DeviceID: target.DeviceID, Address: target.Address, Object: object, Property: property,
		ArrayIndex: uint32(*arrayIndex), Priority: uint8(*priority), Value: value,
		ValueType: valueTypeName, DryRun: !*yes,
	}
	if !*yes {
		return renderWritePlan(a.out, cfg.Output.Format, plan)
	}
	client, _, err := a.open(common)
	if err != nil {
		return err
	}
	defer client.Close()
	if err := client.WriteProperty(bacnetclient.WriteRequest{
		Target: target, Object: object, Property: property, ArrayIndex: uint32(*arrayIndex), Priority: uint8(*priority), Value: value,
	}); err != nil {
		return err
	}
	plan.DryRun = false
	return renderWritePlan(a.out, cfg.Output.Format, plan)
}

func (a *App) watch(args []string) error {
	fs := a.newFlagSet("watch")
	common := addCommonFlags(fs)
	targetFlags := addTargetFlags(fs)
	objectText := fs.String("object", "", "BACnet object as TYPE:INSTANCE")
	propertyText := fs.String("property", "present-value", "property name or numeric identifier")
	arrayIndex := fs.Uint("array-index", uint(^uint32(0)), "array index; defaults to BACnet ARRAY_ALL")
	interval := fs.Duration("interval", time.Second, "poll interval")
	count := fs.Int("count", 0, "number of reads; zero means until interrupted")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *interval <= 0 || *count < 0 {
		return &usageError{message: "--interval must be positive and --count must not be negative"}
	}
	target, err := targetFlags.target()
	if err != nil {
		return err
	}
	object, err := bacnetclient.ParseObjectIdentifier(*objectText)
	if err != nil {
		return err
	}
	property, err := bacnetclient.ParsePropertyIdentifier(*propertyText)
	if err != nil {
		return err
	}
	client, cfg, err := a.open(common)
	if err != nil {
		return err
	}
	defer client.Close()
	format := output.NormaliseFormat(cfg.Output.Format)
	if format == output.FormatTable || format == output.FormatJSON {
		format = output.FormatText
	}
	if err := output.ValidateStreamFormat(format); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()
	csvHeader := true
	for n := 0; *count == 0 || n < *count; n++ {
		value, readErr := client.ReadProperty(target, object, property, uint32(*arrayIndex))
		if readErr != nil {
			return readErr
		}
		if err := renderPropertyStream(a.out, format, value, csvHeader); err != nil {
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

func (a *App) routers(args []string) error {
	fs := a.newFlagSet("routers")
	common := addCommonFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	client, cfg, err := a.open(common)
	if err != nil {
		return err
	}
	defer client.Close()
	routers, err := client.Routers()
	if err != nil {
		return err
	}
	if output.NormaliseFormat(cfg.Output.Format) == output.FormatJSON {
		return output.WriteJSON(a.out, routers)
	}
	for _, router := range routers {
		if err := output.WriteText(a.out, router); err != nil {
			return err
		}
	}
	return nil
}

func normaliseGlobalFlags(args []string) ([]string, error) {
	if len(args) == 0 {
		return args, nil
	}
	var globals []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			if i+1 >= len(args) {
				return nil, errors.New("command is required after --")
			}
			return appendCommandGlobals(args[i+1:], globals), nil
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			return appendCommandGlobals(args[i:], globals), nil
		}
		if arg == "--help" || arg == "-h" || arg == "--version" || arg == "-v" {
			return args[i:], nil
		}
		name, inlineValue, hasInlineValue := strings.Cut(arg, "=")
		switch name {
		case "--verbose", "--debug":
			if hasInlineValue {
				return nil, fmt.Errorf("%s does not take a value", name)
			}
			globals = append(globals, name)
		case "--config", "--profile", "--interface", "--local-ip", "--port", "--subnet-cidr", "--timeout", "--format":
			value := inlineValue
			if !hasInlineValue {
				i++
				if i >= len(args) || strings.HasPrefix(args[i], "-") {
					return nil, fmt.Errorf("%s requires a value", name)
				}
				value = args[i]
			}
			if value == "" {
				return nil, fmt.Errorf("%s requires a value", name)
			}
			globals = append(globals, name, value)
		default:
			return nil, fmt.Errorf("unknown global flag %q", name)
		}
	}
	return nil, errors.New("command is required")
}

func appendCommandGlobals(args []string, globals []string) []string {
	if len(args) == 0 || len(globals) == 0 || !commandSupportsGlobals(args[0]) {
		return args
	}
	out := make([]string, 0, len(args)+len(globals))
	out = append(out, args[0])
	out = append(out, globals...)
	out = append(out, args[1:]...)
	return out
}

func commandSupportsGlobals(command string) bool {
	switch command {
	case "validate-config", "test-connection", "status", "discover", "read", "objects", "write", "watch", "routers":
		return true
	default:
		return false
	}
}
