package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/DishanRajapaksha/bacnet-cli/internal/bacnetclient"
	"github.com/DishanRajapaksha/bacnet-cli/internal/config"
	"github.com/DishanRajapaksha/bacnet-cli/internal/output"
	"gopkg.in/yaml.v3"
)

type inventoryRecord struct {
	Timestamp                  time.Time `json:"timestamp"`
	DeviceID                   int       `json:"device_id"`
	Address                    string    `json:"address"`
	Port                       int       `json:"port"`
	NetworkNumber              int       `json:"network_number"`
	MSTPMAC                    int       `json:"mstp_mac,omitempty"`
	VendorID                   uint32    `json:"vendor_id"`
	ObjectName                 string    `json:"object_name,omitempty"`
	VendorName                 string    `json:"vendor_name,omitempty"`
	ModelName                  string    `json:"model_name,omitempty"`
	FirmwareRevision           string    `json:"firmware_revision,omitempty"`
	ApplicationSoftwareVersion string    `json:"application_software_version,omitempty"`
	Location                   string    `json:"location,omitempty"`
	ProtocolVersion            string    `json:"protocol_version,omitempty"`
	ProtocolRevision           string    `json:"protocol_revision,omitempty"`
	DatabaseRevision           string    `json:"database_revision,omitempty"`
	MaxAPDU                    uint32    `json:"max_apdu"`
	Segmentation               uint32    `json:"segmentation"`
	Error                      string    `json:"error,omitempty"`
}

type discoveryFlagValues struct {
	low             int
	high            int
	network         uint
	globalBroadcast bool
	identify        bool
	failFast        bool
}

func addInventoryDiscoveryFlags(fs *flag.FlagSet) *discoveryFlagValues {
	values := &discoveryFlagValues{}
	fs.IntVar(&values.low, "low", 0, "lowest device instance")
	fs.IntVar(&values.high, "high", 4194303, "highest device instance")
	fs.UintVar(&values.network, "network", 0, "BACnet network number")
	fs.BoolVar(&values.globalBroadcast, "global-broadcast", true, "use a global broadcast")
	fs.BoolVar(&values.identify, "identify", true, "read common device identity properties")
	fs.BoolVar(&values.failFast, "fail-fast", false, "stop after the first identity failure")
	return values
}

func (v *discoveryFlagValues) validate() error {
	if v.low < 0 || v.high < v.low || v.high > 4194303 {
		return fmt.Errorf("%w: discovery range must satisfy 0 <= low <= high <= 4194303", bacnetclient.ErrValidation)
	}
	if v.network > 65535 {
		return fmt.Errorf("%w: network must be between 0 and 65535", bacnetclient.ErrValidation)
	}
	return nil
}

func (v *discoveryFlagValues) options() bacnetclient.DiscoveryOptions {
	return bacnetclient.DiscoveryOptions{
		Low:             v.low,
		High:            v.high,
		GlobalBroadcast: v.globalBroadcast,
		NetworkNumber:   uint16(v.network),
	}
}

func (a *App) inventory(args []string) error {
	fs := a.newFlagSet("inventory")
	common := addCommonFlags(fs)
	discovery := addInventoryDiscoveryFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return &usageError{message: "inventory does not accept positional arguments"}
	}
	if err := discovery.validate(); err != nil {
		return err
	}
	client, cfg, err := a.open(common)
	if err != nil {
		return err
	}
	defer client.Close()

	records, failures, err := collectInventory(client, discovery.options(), discovery.identify, discovery.failFast)
	if err != nil {
		return err
	}
	if err := renderInventory(a.out, cfg.Output.Format, records); err != nil {
		return err
	}
	if failures > 0 {
		return fmt.Errorf("%w: identity inspection failed for %d of %d discovered devices", bacnetclient.ErrRequest, failures, len(records))
	}
	return nil
}

func (a *App) generateConfig(args []string) error {
	fs := a.newFlagSet("generate-config")
	common := addCommonFlags(fs)
	discovery := addInventoryDiscoveryFlags(fs)
	outputPath := fs.String("output", "-", "output YAML path; use - for stdout")
	force := fs.Bool("force", false, "overwrite an existing output file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return &usageError{message: "generate-config does not accept positional arguments"}
	}
	if err := discovery.validate(); err != nil {
		return err
	}
	if strings.TrimSpace(*outputPath) == "" {
		return fmt.Errorf("%w: --output must not be empty", config.ErrConfig)
	}
	if *outputPath != "-" && !*force {
		if _, err := os.Stat(*outputPath); err == nil {
			return fmt.Errorf("%w: %s already exists; use --force", config.ErrConfig, *outputPath)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%w: inspect output %q: %v", config.ErrConfig, *outputPath, err)
		}
	}

	client, cfg, err := a.open(common)
	if err != nil {
		return err
	}
	defer client.Close()

	records, failures, err := collectInventory(client, discovery.options(), discovery.identify, discovery.failFast)
	if err != nil {
		return err
	}
	generated := config.Config{
		Connection: cfg.Connection,
		Output:     config.OutputConfig{Format: "table"},
		Devices:    inventoryDeviceConfigs(records),
	}
	data, err := yaml.Marshal(config.FileConfig{Config: generated})
	if err != nil {
		return fmt.Errorf("%w: encode generated config: %v", output.ErrOutput, err)
	}
	if *outputPath == "-" {
		if _, err := a.out.Write(data); err != nil {
			return fmt.Errorf("%w: write generated config: %v", output.ErrOutput, err)
		}
	} else {
		if err := os.WriteFile(*outputPath, data, 0o600); err != nil {
			return fmt.Errorf("%w: write generated config %q: %v", config.ErrConfig, *outputPath, err)
		}
		fmt.Fprintln(a.out, *outputPath)
	}
	if failures > 0 {
		return fmt.Errorf("%w: generated config with fallback names after %d identity failures", bacnetclient.ErrRequest, failures)
	}
	return nil
}

func collectInventory(client bacnetclient.Client, options bacnetclient.DiscoveryOptions, identify bool, failFast bool) ([]inventoryRecord, int, error) {
	devices, err := client.Discover(options)
	if err != nil {
		return nil, 0, err
	}
	if len(devices) == 0 {
		return nil, 0, fmt.Errorf("%w: no BACnet devices answered Who-Is", bacnetclient.ErrConnection)
	}
	records := make([]inventoryRecord, 0, len(devices))
	failures := 0
	for _, device := range devices {
		record := inventoryRecordFromDevice(device)
		if identify {
			identity, identifyErr := client.Identify(targetFromDiscoveredDevice(device))
			if identifyErr != nil {
				record.Error = identifyErr.Error()
				failures++
			} else {
				applyIdentity(&record, identity)
			}
		}
		records = append(records, record)
		if record.Error != "" && failFast {
			break
		}
	}
	sort.Slice(records, func(i, j int) bool { return records[i].DeviceID < records[j].DeviceID })
	return records, failures, nil
}

func inventoryRecordFromDevice(device bacnetclient.Device) inventoryRecord {
	return inventoryRecord{
		Timestamp:     time.Now().UTC(),
		DeviceID:      device.DeviceID,
		Address:       device.Address,
		Port:          device.Port,
		NetworkNumber: device.NetworkNumber,
		MSTPMAC:       device.MSTPMAC,
		VendorID:      device.VendorID,
		MaxAPDU:       device.MaxAPDU,
		Segmentation:  device.Segmentation,
	}
}

func targetFromDiscoveredDevice(device bacnetclient.Device) bacnetclient.Target {
	var mstp *int
	if device.NetworkNumber != 0 {
		value := device.MSTPMAC
		mstp = &value
	}
	return bacnetclient.Target{
		DeviceID:      device.DeviceID,
		Address:       device.Address,
		Port:          device.Port,
		NetworkNumber: device.NetworkNumber,
		MSTPMAC:       mstp,
		MaxAPDU:       device.MaxAPDU,
		Segmentation:  device.Segmentation,
	}
}

func applyIdentity(record *inventoryRecord, identity bacnetclient.DeviceIdentity) {
	values := make(map[string]string, len(identity.Fields))
	for _, field := range identity.Fields {
		if field.Error == "" {
			values[field.Property.Name] = formatValue(field.Value)
		}
	}
	record.ObjectName = values["object-name"]
	record.VendorName = values["vendor-name"]
	record.ModelName = values["model-name"]
	record.FirmwareRevision = values["firmware-revision"]
	record.ApplicationSoftwareVersion = values["application-software-version"]
	record.Location = values["location"]
	record.ProtocolVersion = values["protocol-version"]
	record.ProtocolRevision = values["protocol-revision"]
	record.DatabaseRevision = values["database-revision"]
}

func renderInventory(w io.Writer, format string, records []inventoryRecord) error {
	format = output.NormaliseFormat(format)
	if err := output.ValidateSnapshotFormat(format); err != nil {
		return err
	}
	if format == output.FormatJSON {
		return output.WriteJSON(w, records)
	}
	rows := make([][]string, 0, len(records))
	for _, record := range records {
		rows = append(rows, []string{
			strconv.Itoa(record.DeviceID), record.Address, strconv.Itoa(record.Port),
			strconv.Itoa(record.NetworkNumber), strconv.Itoa(record.MSTPMAC),
			record.ObjectName, record.VendorName, record.ModelName,
			record.FirmwareRevision, record.Location,
			strconv.FormatUint(uint64(record.VendorID), 10),
			strconv.FormatUint(uint64(record.MaxAPDU), 10),
			strconv.FormatUint(uint64(record.Segmentation), 10), record.Error,
		})
	}
	headers := []string{"DEVICE", "ADDRESS", "PORT", "NETWORK", "MSTP", "NAME", "VENDOR", "MODEL", "FIRMWARE", "LOCATION", "VENDOR_ID", "MAX_APDU", "SEGMENTATION", "ERROR"}
	return renderRows(w, format, headers, rows)
}

func inventoryDeviceConfigs(records []inventoryRecord) []config.DeviceConfig {
	used := make(map[string]struct{}, len(records))
	devices := make([]config.DeviceConfig, 0, len(records))
	for _, record := range records {
		name := generatedDeviceName(record.ObjectName, record.DeviceID, used)
		segmentation := record.Segmentation
		device := config.DeviceConfig{
			Name:         name,
			DeviceID:     record.DeviceID,
			Address:      record.Address,
			Port:         record.Port,
			Network:      record.NetworkNumber,
			MaxAPDU:      record.MaxAPDU,
			Segmentation: &segmentation,
		}
		if record.NetworkNumber != 0 {
			mstp := record.MSTPMAC
			device.MSTPMAC = &mstp
		}
		devices = append(devices, device)
	}
	return devices
}

var generatedNameSeparators = regexp.MustCompile(`_+`)

func generatedDeviceName(objectName string, deviceID int, used map[string]struct{}) string {
	var builder strings.Builder
	for _, r := range strings.TrimSpace(objectName) {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			builder.WriteRune(unicode.ToLower(r))
		default:
			builder.WriteByte('_')
		}
	}
	name := strings.Trim(generatedNameSeparators.ReplaceAllString(builder.String(), "_"), "_")
	if name == "" {
		name = fmt.Sprintf("device_%d", deviceID)
	}
	if first := rune(name[0]); unicode.IsDigit(first) {
		name = "device_" + name
	}
	candidate := name
	if _, exists := used[candidate]; exists {
		candidate = fmt.Sprintf("%s_%d", name, deviceID)
	}
	for suffix := 2; ; suffix++ {
		if _, exists := used[candidate]; !exists {
			used[candidate] = struct{}{}
			return candidate
		}
		candidate = fmt.Sprintf("%s_%d_%d", name, deviceID, suffix)
	}
}
