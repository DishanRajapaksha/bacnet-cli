package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/DishanRajapaksha/industrial-cli-kit/completion"
)

func TestRegistryMatchesDispatcher(t *testing.T) {
	dispatched := []string{
		"init-config", "validate-config", "test-connection", "status", "discover", "read",
		"objects", "write", "watch", "routers", "completions", "help", "version",
	}
	registered := map[string]bool{}
	for _, command := range cliRegistry.Commands {
		if registered[command.Name] {
			t.Fatalf("duplicate registry command %q", command.Name)
		}
		registered[command.Name] = true
	}
	for _, name := range dispatched {
		if !registered[name] {
			t.Errorf("dispatcher command %q is not registered", name)
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
			t.Errorf("registered command %q is not dispatched", name)
		}
	}
}

func TestRegistryDoesNotAdvertiseGhostCommands(t *testing.T) {
	ghosts := map[string]bool{
		"devices": true, "inventory": true, "generate-config": true,
		"read-points": true, "watch-points": true,
	}
	for _, command := range cliRegistry.Commands {
		if ghosts[command.Name] {
			t.Errorf("registry advertises unsupported command %q", command.Name)
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
		normalised, err := normaliseGlobalFlags(args)
		if err != nil {
			t.Errorf("registered global flag --%s is rejected: %v", global.Name, err)
			continue
		}
		if len(normalised) == 0 || normalised[0] != "status" {
			t.Errorf("normalising --%s produced %v", global.Name, normalised)
		}
	}
}

func TestGeneratedCompletionsContainCommandFlags(t *testing.T) {
	var out bytes.Buffer
	if err := completion.Write(&out, "bash", cliRegistry); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"discover", "--low", "--device-id", "--object", "--property", "--yes"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("completion output missing %q", want)
		}
	}
}
