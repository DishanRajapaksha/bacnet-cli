package cli

import sharedhelp "github.com/DishanRajapaksha/industrial-cli-kit/help"

func (a *App) writeRegistryUsage() {
	_ = sharedhelp.Write(a.out, cliRegistry, sharedhelp.Options{
		Description: "bacnet-cli is a script-friendly BACnet/IP command-line client.",
		Usage: []string{
			"bacnet-cli [global flags] <command> [flags]",
		},
		Examples: []string{
			"bacnet-cli init-config",
			"bacnet-cli validate-config --profile local",
			"bacnet-cli discover --low 0 --high 4194303",
			"bacnet-cli test-connection --device-id 1234",
			"bacnet-cli read --device-id 1234 --object analog-input:1 --property present-value",
			"bacnet-cli objects --device-id 1234",
			"bacnet-cli write --device-id 1234 --object analog-output:1 --property present-value --type float32 --value 21.5 --yes",
			"bacnet-cli watch --device-id 1234 --object analog-input:1 --property present-value --interval 2s --format jsonl",
			"bacnet-cli routers",
			"bacnet-cli completions zsh",
		},
	})
}
