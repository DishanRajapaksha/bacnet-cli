package cli

import "github.com/DishanRajapaksha/industrial-cli-kit/command"

var cliRegistry = command.Registry{
	Binary: appName,
	GlobalFlags: []command.Flag{
		{Name: "config", TakesValue: true, Summary: "YAML config file"},
		{Name: "profile", TakesValue: true, Summary: "config profile name"},
		{Name: "interface", TakesValue: true, Summary: "local network interface"},
		{Name: "local-ip", TakesValue: true, Summary: "local IPv4 address"},
		{Name: "port", TakesValue: true, Summary: "local BACnet/IP UDP port"},
		{Name: "subnet-cidr", TakesValue: true, Summary: "local IPv4 subnet prefix"},
		{Name: "timeout", TakesValue: true, Summary: "request timeout"},
		{Name: "format", TakesValue: true, Summary: "output format"},
		{Name: "verbose", Summary: "print connection decisions"},
		{Name: "debug", Summary: "enable protocol debug logging"},
	},
	Commands: []command.Command{
		{Name: "init-config", Summary: "Write a starter YAML config file", Flags: registryFlags("output", "force")},
		{Name: "validate-config", Summary: "Validate local config without opening a socket"},
		{Name: "test-connection", Summary: "Open BACnet/IP and require a device response", Flags: registryFlags("device-id")},
		{Name: "status", Summary: "Alias for test-connection", Flags: registryFlags("device-id")},
		{Name: "discover", Summary: "Send Who-Is and list I-Am responses", Flags: registryFlags("low", "high", "network", "global-broadcast")},
		{Name: "read", Summary: "Read one BACnet property", Flags: append(targetRegistryFlags(), registryFlags("object", "property", "array-index")...)},
		{Name: "objects", Summary: "Read a device object list", Flags: targetRegistryFlags()},
		{Name: "write", Summary: "Write one BACnet property", Flags: append(targetRegistryFlags(), registryFlags("object", "property", "array-index", "value", "type", "priority", "null", "dry-run", "yes")...)},
		{Name: "watch", Summary: "Poll one BACnet property", Flags: append(targetRegistryFlags(), registryFlags("object", "property", "array-index", "interval", "count")...)},
		{Name: "routers", Summary: "Send Who-Is-Router-To-Network"},

		{Name: "devices", Summary: "List configured named BACnet devices"},
		{Name: "points", Summary: "List configured named BACnet points"},
		{Name: "read-point", Summary: "Read a configured named point", LeadingArgs: 1},
		{Name: "read-points", Summary: "Read selected configured points", Flags: registryFlags("point", "device", "fail-fast")},
		{Name: "write-point", Summary: "Write a configured named point", LeadingArgs: 1, Flags: registryFlags("value", "null", "dry-run", "yes", "priority")},
		{Name: "watch-point", Summary: "Poll a configured named point", LeadingArgs: 1, Flags: registryFlags("interval", "duration", "count")},
		{Name: "watch-points", Summary: "Poll selected configured points", Flags: registryFlags("point", "device", "interval", "duration", "count", "fail-fast")},
		{Name: "identify", Summary: "Read standard device identity properties", LeadingArgs: 1, Flags: targetRegistryFlags()},
		{Name: "inventory", Summary: "Discover devices and collect identity metadata", Flags: inventoryRegistryFlags()},
		{Name: "generate-config", Summary: "Discover devices and emit YAML configuration", Flags: append(inventoryRegistryFlags(), registryFlags("output", "force")...)},
		{Name: "object-types", Summary: "List supported object type names", Flags: registryFlags("format")},
		{Name: "properties", Summary: "List supported property names", Flags: registryFlags("format")},

		{Name: "completions", Summary: "Generate Bash or Zsh completion scripts", LeadingArgs: 1},
		{Name: "help", Summary: "Print help"},
		{Name: "version", Summary: "Print version information"},
	},
}

func targetRegistryFlags() []command.Flag {
	return registryFlags("device-id", "device-address", "device-port", "network", "mstp-mac", "max-apdu", "segmentation")
}

func inventoryRegistryFlags() []command.Flag {
	return registryFlags("low", "high", "network", "global-broadcast", "identify", "fail-fast")
}

func registryFlags(names ...string) []command.Flag {
	boolean := map[string]bool{
		"debug":            true,
		"dry-run":          true,
		"fail-fast":        true,
		"force":            true,
		"global-broadcast": true,
		"identify":         true,
		"null":             true,
		"verbose":          true,
		"yes":              true,
	}
	flags := make([]command.Flag, 0, len(names))
	for _, name := range names {
		flags = append(flags, command.Flag{Name: name, TakesValue: !boolean[name]})
	}
	return flags
}
