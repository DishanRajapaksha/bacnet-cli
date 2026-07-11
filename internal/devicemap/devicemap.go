package devicemap

import (
	"fmt"
	"sort"
	"strings"

	"github.com/DishanRajapaksha/bacnet-cli/internal/bacnetclient"
	"github.com/DishanRajapaksha/bacnet-cli/internal/config"
)

const arrayAll = ^uint32(0)

type ResolvedPoint struct {
	Point      config.PointConfig
	Device     config.DeviceConfig
	Target     bacnetclient.Target
	Object     bacnetclient.ObjectIdentifier
	Property   bacnetclient.PropertyIdentifier
	ArrayIndex uint32
	Priority   uint8
}

func FindDevice(devices []config.DeviceConfig, name string) (config.DeviceConfig, error) {
	for _, device := range devices {
		if device.Name == name {
			return device, nil
		}
	}
	return config.DeviceConfig{}, fmt.Errorf("%w: device %q not found", config.ErrConfig, name)
}

func FindPoint(points []config.PointConfig, name string) (config.PointConfig, error) {
	for _, point := range points {
		if point.Name == name {
			return point, nil
		}
	}
	return config.PointConfig{}, fmt.Errorf("%w: point %q not found", config.ErrConfig, name)
}

func ResolveDevice(device config.DeviceConfig) bacnetclient.Target {
	port := device.Port
	if port == 0 {
		port = config.DefaultPort
	}
	maxAPDU := device.MaxAPDU
	if maxAPDU == 0 {
		maxAPDU = 1476
	}
	segmentation := uint32(3)
	if device.Segmentation != nil {
		segmentation = *device.Segmentation
	}
	return bacnetclient.Target{
		DeviceID:      device.DeviceID,
		Address:       device.Address,
		Port:          port,
		NetworkNumber: device.Network,
		MSTPMAC:       device.MSTPMAC,
		MaxAPDU:       maxAPDU,
		Segmentation:  segmentation,
	}
}

func ResolvePoint(cfg config.Config, name string) (ResolvedPoint, error) {
	point, err := FindPoint(cfg.Points, name)
	if err != nil {
		return ResolvedPoint{}, err
	}
	device, err := FindDevice(cfg.Devices, point.Device)
	if err != nil {
		return ResolvedPoint{}, err
	}
	object, err := bacnetclient.ParseObjectIdentifier(point.Object)
	if err != nil {
		return ResolvedPoint{}, err
	}
	propertyName := point.Property
	if strings.TrimSpace(propertyName) == "" {
		propertyName = "present-value"
	}
	property, err := bacnetclient.ParsePropertyIdentifier(propertyName)
	if err != nil {
		return ResolvedPoint{}, err
	}
	arrayIndex := uint32(arrayAll)
	if point.ArrayIndex != nil {
		arrayIndex = *point.ArrayIndex
	}
	priority := point.Priority
	if priority == 0 {
		priority = 16
	}
	return ResolvedPoint{
		Point:      point,
		Device:     device,
		Target:     ResolveDevice(device),
		Object:     object,
		Property:   property,
		ArrayIndex: arrayIndex,
		Priority:   priority,
	}, nil
}

func ListDevices(devices []config.DeviceConfig) []config.DeviceConfig {
	out := append([]config.DeviceConfig(nil), devices...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func ListPoints(points []config.PointConfig) []config.PointConfig {
	out := append([]config.PointConfig(nil), points...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func ApplyPoint(point config.PointConfig, value bacnetclient.PropertyValue) bacnetclient.PropertyValue {
	value.Point = point.Name
	value.Unit = point.Unit
	return value
}
