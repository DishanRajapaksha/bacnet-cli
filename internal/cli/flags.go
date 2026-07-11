package cli

import (
	"flag"
	"time"

	"github.com/DishanRajapaksha/bacnet-cli/internal/bacnetclient"
	"github.com/DishanRajapaksha/bacnet-cli/internal/config"
)

type commonFlagValues struct {
	configPath string
	profile    string
	iface      string
	localIP    string
	port       int
	subnetCIDR int
	timeout    time.Duration
	format     string
	verbose    bool
	debug      bool
}

func addCommonFlags(fs *flag.FlagSet) *commonFlagValues {
	values := &commonFlagValues{}
	fs.StringVar(&values.configPath, "config", config.DefaultConfigPath, "YAML config file")
	fs.StringVar(&values.profile, "profile", "", "config profile name")
	fs.StringVar(&values.iface, "interface", "", "local network interface, for example en0 or eth0")
	fs.StringVar(&values.localIP, "local-ip", "", "local IPv4 address; alternative to --interface")
	fs.IntVar(&values.port, "port", 0, "local BACnet/IP UDP port")
	fs.IntVar(&values.subnetCIDR, "subnet-cidr", -1, "local IPv4 subnet prefix length")
	fs.DurationVar(&values.timeout, "timeout", 0, "request timeout")
	fs.StringVar(&values.format, "format", "", "output format")
	fs.BoolVar(&values.verbose, "verbose", false, "print high-level connection decisions")
	fs.BoolVar(&values.debug, "debug", false, "enable lower-level debug logging")
	return values
}

func (v *commonFlagValues) loadConfig() (config.Config, error) {
	overrides := config.Overrides{Interface: v.iface, LocalIP: v.localIP, Format: v.format}
	if v.port != 0 {
		overrides.Port = &v.port
	}
	if v.subnetCIDR >= 0 {
		overrides.SubnetCIDR = &v.subnetCIDR
	}
	if v.timeout != 0 {
		overrides.Timeout = &v.timeout
	}
	return config.LoadForProfile(v.configPath, v.profile, overrides)
}

type targetFlagValues struct {
	deviceID      int
	deviceAddress string
	devicePort    int
	network       int
	mstpMAC       int
	maxAPDU       uint
	segmentation  uint
}

func addTargetFlags(fs *flag.FlagSet) *targetFlagValues {
	values := &targetFlagValues{deviceID: -1, mstpMAC: -1, maxAPDU: 1476, segmentation: 3}
	fs.IntVar(&values.deviceID, "device-id", -1, "BACnet device instance")
	fs.StringVar(&values.deviceAddress, "device-address", "", "device IPv4 address; omitted to discover by device id")
	fs.IntVar(&values.devicePort, "device-port", config.DefaultPort, "device BACnet/IP UDP port")
	fs.IntVar(&values.network, "network", 0, "BACnet network number")
	fs.IntVar(&values.mstpMAC, "mstp-mac", -1, "routed MS/TP MAC address")
	fs.UintVar(&values.maxAPDU, "max-apdu", 1476, "device maximum APDU length")
	fs.UintVar(&values.segmentation, "segmentation", 3, "device segmentation mode")
	return values
}

func (v *targetFlagValues) target() (bacnetclient.Target, error) {
	if v.deviceID < 0 {
		return bacnetclient.Target{}, &usageError{message: "--device-id is required"}
	}
	var mstp *int
	if v.mstpMAC >= 0 {
		value := v.mstpMAC
		mstp = &value
	}
	return bacnetclient.Target{
		DeviceID:      v.deviceID,
		Address:       v.deviceAddress,
		Port:          v.devicePort,
		NetworkNumber: v.network,
		MSTPMAC:       mstp,
		MaxAPDU:       uint32(v.maxAPDU),
		Segmentation:  uint32(v.segmentation),
	}, nil
}
