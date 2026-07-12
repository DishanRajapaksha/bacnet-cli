package cli

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/DishanRajapaksha/industrial-cli-kit/command"
	"github.com/DishanRajapaksha/industrial-cli-kit/completion"
)

func TestRegistryMatchesPublicDispatcher(t *testing.T) {
	dispatched := []string{
		"init-config", "validate-config", "test-connection", "status", "discover", "read",
		"objects", "write", "watch", "routers", "completions", "help", "version",
		"devices", "points", "read-point", "read-points", "write-point", "watch-point",
		"watch-points", "identify", "inventory", "generate-config", "object-types", "properties",
	}
	registered := map[string]bool{}
	for _, registeredCommand := range cliRegistry.Commands {
		if registered[registeredCommand.Name] {
			t.Fatalf("duplicate registry command %q", registeredCommand.Name)
		}
		registered[registeredCommand.Name] = true
	}
	for _, name := range dispatched {
		if !registered[name] {
			t.Errorf("public dispatcher command %q is not registered", name)
		}
	}
	for name := range registered {
		found := false
		for _, candidate := range dispatched {
			if candidate == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("registered command %q is not publicly dispatched", name)
		}
	}
}

func TestRegistryGlobalFlagsMatchNormalizer(t *testing.T) {
	for _, global := range cliRegistry.GlobalFlags {
		args := []string{"--" + global.Name}
		if global.TakesValue {
			args = append(args, "value")
		}
		args = append(args, "status")
		normalised, err := normaliseExtendedGlobalFlags(args)
		if err != nil {
			t.Errorf("registered global flag --%s is rejected: %v", global.Name, err)
			continue
		}
		if len(normalised) == 0 || normalised[0] != "status" {
			t.Errorf("normalising --%s produced %v", global.Name, normalised)
		}
	}
}

func TestRegistryNormalizerPreservesNamedArguments(t *testing.T) {
	got, err := normaliseExtendedGlobalFlags([]string{
		"--profile", "local", "read-point", "supply_air_temperature", "--format", "json",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"read-point", "supply_air_temperature", "--profile", "local", "--format", "json",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normaliseExtendedGlobalFlags() = %#v, want %#v", got, want)
	}

	got, err = normaliseExtendedGlobalFlags([]string{
		"--timeout", "3s", "identify", "ahu", "--device-id", "1234",
	})
	if err != nil {
		t.Fatal(err)
	}
	want = []string{"identify", "ahu", "--timeout", "3s", "--device-id", "1234"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normaliseExtendedGlobalFlags() = %#v, want %#v", got, want)
	}
}

func TestRegistryCapturesExtendedFlagsAndSafety(t *testing.T) {
	write := registryCommand(t, "write")
	assertFlag(t, write.Flags, "yes", false)
	assertFlag(t, write.Flags, "dry-run", false)
	assertFlag(t, write.Flags, "null", false)

	writePoint := registryCommand(t, "write-point")
	if writePoint.LeadingArgs != 1 {
		t.Fatalf("write-point LeadingArgs=%d, want 1", writePoint.LeadingArgs)
	}
	for _, name := range []string{"value", "priority"} {
		assertFlag(t, writePoint.Flags, name, true)
	}
	for _, name := range []string{"yes", "dry-run", "null"} {
		assertFlag(t, writePoint.Flags, name, false)
	}

	watchPoint := registryCommand(t, "watch-point")
	if watchPoint.LeadingArgs != 1 {
		t.Fatalf("watch-point LeadingArgs=%d, want 1", watchPoint.LeadingArgs)
	}
	for _, name := range []string{"interval", "duration", "count"} {
		assertFlag(t, watchPoint.Flags, name, true)
	}

	inventory := registryCommand(t, "inventory")
	for _, name := range []string{"global-broadcast", "identify", "fail-fast"} {
		assertFlag(t, inventory.Flags, name, false)
	}
	generate := registryCommand(t, "generate-config")
	assertFlag(t, generate.Flags, "output", true)
	assertFlag(t, generate.Flags, "force", false)

	readPoints := registryCommand(t, "read-points")
	assertFlag(t, readPoints.Flags, "point", true)
	assertFlag(t, readPoints.Flags, "device", true)
	assertFlag(t, readPoints.Flags, "fail-fast", false)
}

func TestGeneratedCompletionsContainFullPublicSurface(t *testing.T) {
	var out bytes.Buffer
	if err := completion.Write(&out, "bash", cliRegistry); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"devices", "read-points", "watch-points", "inventory", "generate-config",
		"object-types", "properties", "--dry-run", "--fail-fast", "--duration",
		"--output", "complete -F _bacnet_cli_completion bacnet-cli",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("completion output missing %q", want)
		}
	}
}

func registryCommand(t *testing.T, name string) command.Command {
	t.Helper()
	for _, registered := range cliRegistry.Commands {
		if registered.Name == name {
			return registered
		}
	}
	t.Fatalf("registry command %q not found", name)
	return command.Command{}
}

func assertFlag(t *testing.T, flags []command.Flag, name string, takesValue bool) {
	t.Helper()
	for _, flag := range flags {
		if flag.Name == name {
			if flag.TakesValue != takesValue {
				t.Fatalf("flag --%s TakesValue=%v, want %v", name, flag.TakesValue, takesValue)
			}
			return
		}
	}
	t.Fatalf("flag --%s not found", name)
}
