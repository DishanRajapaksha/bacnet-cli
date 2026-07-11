package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/DishanRajapaksha/bacnet-cli/internal/bacnetclient"
)

func MainV2() {
	code := NewApp(os.Stdout, os.Stderr).RunV2(os.Args[1:])
	if code != 0 {
		os.Exit(code)
	}
}

func (a *App) RunV2(args []string) int {
	normalised, err := normaliseExtendedGlobalFlags(args)
	if err != nil {
		fmt.Fprintln(a.err, err)
		return exitConfigError
	}
	args = normalised
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		a.printUsage()
		a.printExtendedUsage()
		return exitSuccess
	}

	switch args[0] {
	case "devices":
		err = a.devices(args[1:])
	case "points":
		err = a.points(args[1:])
	case "read-point":
		err = a.readPoint(args[1:])
	case "read-points":
		err = a.readPoints(args[1:])
	case "write-point":
		err = a.writePoint(args[1:])
	case "watch-point":
		err = a.watchPoint(args[1:])
	case "watch-points":
		err = a.watchPoints(args[1:])
	case "identify":
		err = a.identify(args[1:])
	case "object-types":
		err = a.objectTypes(args[1:])
	case "properties":
		err = a.properties(args[1:])
	case "write":
		err = a.writeV2(args[1:])
	default:
		return a.Run(args)
	}
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitSuccess
		}
		fmt.Fprintln(a.err, err)
		return mapExtendedExitCode(err)
	}
	return exitSuccess
}

func mapExtendedExitCode(err error) int {
	if errors.Is(err, bacnetclient.ErrWriteRejected) {
		return exitWriteRejected
	}
	return mapExitCode(err)
}

func (a *App) printExtendedUsage() {
	fmt.Fprintln(a.out, `
Additional commands:
  devices          List configured named BACnet devices
  points           List configured named BACnet points
  read-point       Read a configured named point
  read-points      Read selected or all configured points in one session
  write-point      Write a configured named point; dry-run unless --yes is supplied
  watch-point      Poll a configured named point
  watch-points     Poll selected or all configured points in cycles
  identify         Read standard identity properties from a device
  object-types     List supported object type names and aliases
  properties       List supported property names and identifiers

Examples:
  bacnet-cli devices --profile local
  bacnet-cli points --format json
  bacnet-cli read-point supply_air_temperature
  bacnet-cli read-points --device ahu
  bacnet-cli read-points --point supply_air_temperature --point cooling_setpoint --format json
  bacnet-cli write-point cooling_setpoint --value 21.5
  bacnet-cli write-point cooling_setpoint --value 21.5 --yes
  bacnet-cli watch-point supply_air_temperature --interval 2s --format jsonl
  bacnet-cli watch-points --device ahu --interval 5s --format csv
  bacnet-cli identify ahu
  bacnet-cli object-types
  bacnet-cli properties --format csv`)
}

func normaliseExtendedGlobalFlags(args []string) ([]string, error) {
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
			return appendExtendedGlobals(args[i+1:], globals), nil
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			return appendExtendedGlobals(args[i:], globals), nil
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

func appendExtendedGlobals(args []string, globals []string) []string {
	if len(args) == 0 || len(globals) == 0 || !extendedCommandSupportsGlobals(args[0]) {
		return args
	}
	out := make([]string, 0, len(args)+len(globals))
	out = append(out, args[0])
	if extendedCommandTakesNameBeforeFlags(args[0]) && len(args) > 1 && !strings.HasPrefix(args[1], "-") {
		out = append(out, args[1])
		out = append(out, globals...)
		out = append(out, args[2:]...)
		return out
	}
	out = append(out, globals...)
	out = append(out, args[1:]...)
	return out
}

func extendedCommandTakesNameBeforeFlags(command string) bool {
	switch command {
	case "read-point", "write-point", "watch-point", "identify":
		return true
	default:
		return false
	}
}

func extendedCommandSupportsGlobals(command string) bool {
	if commandSupportsGlobals(command) {
		return true
	}
	switch command {
	case "devices", "points", "read-point", "read-points", "write-point", "watch-point", "watch-points", "identify", "object-types", "properties":
		return true
	default:
		return false
	}
}
