package bacnetclient

import (
	"fmt"
	"net"
	"reflect"
	"sort"
	"time"

	"github.com/NubeDev/bacnet"
	"github.com/NubeDev/bacnet/btypes"
	"github.com/NubeDev/bacnet/btypes/null"
	ipbytes "github.com/NubeDev/bacnet/helpers/ipbytes"
)

type Client interface {
	Close() error
	Discover(DiscoveryOptions) ([]Device, error)
	ReadProperty(Target, ObjectIdentifier, PropertyIdentifier, uint32) (PropertyValue, error)
	WriteProperty(WriteRequest) error
	Objects(Target) ([]Object, error)
	Identify(Target) (DeviceIdentity, error)
	Routers() ([]string, error)
}

type Factory interface {
	Open(Options) (Client, error)
}

type NubeFactory struct{}

type nubeClient struct {
	raw bacnet.Client
}

func (NubeFactory) Open(opts Options) (Client, error) {
	iface := opts.Interface
	if iface == "" && opts.LocalIP == "" {
		var err error
		iface, err = firstUsableInterface()
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrConnection, err)
		}
	}
	client, err := bacnet.NewClient(&bacnet.ClientBuilder{
		Interface:  iface,
		Ip:         opts.LocalIP,
		Port:       opts.Port,
		SubnetCIDR: opts.SubnetCIDR,
		MaxPDU:     btypes.MaxAPDU,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: open BACnet/IP socket: %v", ErrConnection, err)
	}
	go client.ClientRun()
	return &nubeClient{raw: client}, nil
}

func firstUsableInterface() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, address := range addresses {
			ip, _, err := net.ParseCIDR(address.String())
			if err == nil && ip.To4() != nil {
				return iface.Name, nil
			}
		}
	}
	return "", fmt.Errorf("no active non-loopback IPv4 interface found; use --interface or --local-ip")
}

func (c *nubeClient) Close() error {
	return c.raw.Close()
}

func (c *nubeClient) Discover(opts DiscoveryOptions) ([]Device, error) {
	devices, err := c.raw.WhoIs(&bacnet.WhoIsOpts{
		Low:             opts.Low,
		High:            opts.High,
		GlobalBroadcast: opts.GlobalBroadcast,
		NetworkNumber:   opts.NetworkNumber,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: Who-Is failed: %v", ErrRequest, err)
	}
	out := make([]Device, 0, len(devices))
	for _, device := range devices {
		out = append(out, fromLibraryDevice(device))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DeviceID < out[j].DeviceID })
	return out, nil
}

func fromLibraryDevice(device btypes.Device) Device {
	return Device{
		DeviceID:      device.DeviceID,
		Address:       device.Ip,
		Port:          device.Port,
		NetworkNumber: device.NetworkNumber,
		MSTPMAC:       device.MacMSTP,
		MaxAPDU:       device.MaxApdu,
		Segmentation:  uint32(device.Segmentation),
		VendorID:      device.Vendor,
	}
}

func (c *nubeClient) resolveTarget(target Target) (btypes.Device, error) {
	if target.DeviceID < 0 {
		return btypes.Device{}, fmt.Errorf("%w: device id must be zero or greater", ErrValidation)
	}
	if target.Address == "" {
		devices, err := c.Discover(DiscoveryOptions{
			Low:             target.DeviceID,
			High:            target.DeviceID,
			GlobalBroadcast: true,
			NetworkNumber:   uint16(target.NetworkNumber),
		})
		if err != nil {
			return btypes.Device{}, err
		}
		for _, device := range devices {
			if device.DeviceID == target.DeviceID {
				return c.libraryDevice(Target{
					DeviceID:      device.DeviceID,
					Address:       device.Address,
					Port:          device.Port,
					NetworkNumber: device.NetworkNumber,
					MaxAPDU:       device.MaxAPDU,
					Segmentation:  device.Segmentation,
				})
			}
		}
		return btypes.Device{}, fmt.Errorf("%w: device %d did not answer Who-Is", ErrConnection, target.DeviceID)
	}
	return c.libraryDevice(target)
}

func (c *nubeClient) libraryDevice(target Target) (btypes.Device, error) {
	port := target.Port
	if port == 0 {
		port = 47808
	}
	mac, err := ipbytes.New(target.Address, uint16(port))
	if err != nil {
		return btypes.Device{}, fmt.Errorf("%w: invalid device address: %v", ErrValidation, err)
	}
	address := btypes.Address{Net: uint16(target.NetworkNumber), Mac: mac, MacLen: uint8(len(mac))}
	if target.MSTPMAC != nil {
		if *target.MSTPMAC < 0 || *target.MSTPMAC > 255 {
			return btypes.Device{}, fmt.Errorf("%w: MSTP MAC must be between 0 and 255", ErrValidation)
		}
		address.Adr = []uint8{uint8(*target.MSTPMAC)}
	}
	maxAPDU := target.MaxAPDU
	if maxAPDU == 0 {
		maxAPDU = 1476
	}
	return btypes.Device{
		ID:            btypes.ObjectID{Type: btypes.DeviceType, Instance: btypes.ObjectInstance(target.DeviceID)},
		DeviceID:      target.DeviceID,
		Ip:            target.Address,
		Port:          port,
		NetworkNumber: target.NetworkNumber,
		MaxApdu:       maxAPDU,
		Segmentation:  btypes.Enumerated(target.Segmentation),
		Addr:          address,
	}, nil
}

func (c *nubeClient) ReadProperty(target Target, object ObjectIdentifier, property PropertyIdentifier, arrayIndex uint32) (PropertyValue, error) {
	device, err := c.resolveTarget(target)
	if err != nil {
		return PropertyValue{}, err
	}
	return c.readPropertyFromDevice(device, target.DeviceID, object, property, arrayIndex)
}

func (c *nubeClient) readPropertyFromDevice(device btypes.Device, deviceID int, object ObjectIdentifier, property PropertyIdentifier, arrayIndex uint32) (PropertyValue, error) {
	request := btypes.PropertyData{Object: btypes.Object{
		ID:         btypes.ObjectID{Type: btypes.ObjectType(object.Type), Instance: btypes.ObjectInstance(object.Instance)},
		Properties: []btypes.Property{{Type: btypes.PropertyType(property.ID), ArrayIndex: arrayIndex}},
	}}
	response, err := c.raw.ReadProperty(device, request)
	if err != nil {
		return PropertyValue{}, fmt.Errorf("%w: read property: %v", ErrRequest, err)
	}
	if len(response.Object.Properties) == 0 {
		return PropertyValue{}, fmt.Errorf("%w: device returned no property value", ErrRequest)
	}
	value := response.Object.Properties[0].Data
	valueType := "<nil>"
	if value != nil {
		valueType = reflect.TypeOf(value).String()
	}
	return PropertyValue{
		Timestamp:  time.Now().UTC(),
		DeviceID:   deviceID,
		Object:     object,
		Property:   property,
		ArrayIndex: arrayIndex,
		Value:      value,
		ValueType:  valueType,
	}, nil
}

func (c *nubeClient) WriteProperty(request WriteRequest) error {
	device, err := c.resolveTarget(request.Target)
	if err != nil {
		return err
	}
	value := request.Value
	if _, ok := value.(NullValue); ok {
		value = null.Null{}
	}
	payload := btypes.PropertyData{Object: btypes.Object{
		ID: btypes.ObjectID{Type: btypes.ObjectType(request.Object.Type), Instance: btypes.ObjectInstance(request.Object.Instance)},
		Properties: []btypes.Property{{
			Type:       btypes.PropertyType(request.Property.ID),
			ArrayIndex: request.ArrayIndex,
			Priority:   btypes.NPDUPriority(request.Priority),
			Data:       value,
		}},
	}}
	if err := c.raw.WriteProperty(device, payload); err != nil {
		return fmt.Errorf("%w: write property: %v", ErrWriteRejected, err)
	}
	return nil
}

func (c *nubeClient) Objects(target Target) ([]Object, error) {
	device, err := c.resolveTarget(target)
	if err != nil {
		return nil, err
	}
	device, err = c.raw.Objects(device)
	if err != nil {
		return nil, fmt.Errorf("%w: read object list: %v", ErrRequest, err)
	}
	objects := device.ObjectSlice()
	out := make([]Object, 0, len(objects))
	for _, object := range objects {
		out = append(out, Object{
			Type:        uint16(object.ID.Type),
			TypeName:    ObjectTypeName(uint16(object.ID.Type)),
			Instance:    uint32(object.ID.Instance),
			Name:        object.Name,
			Description: object.Description,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Type == out[j].Type {
			return out[i].Instance < out[j].Instance
		}
		return out[i].Type < out[j].Type
	})
	return out, nil
}

func (c *nubeClient) Identify(target Target) (DeviceIdentity, error) {
	device, err := c.resolveTarget(target)
	if err != nil {
		return DeviceIdentity{}, err
	}
	identity := DeviceIdentity{
		Timestamp: time.Now().UTC(),
		DeviceID:  target.DeviceID,
		Address:   device.Ip,
	}
	object := ObjectIdentifier{Type: 8, TypeName: "device", Instance: uint32(target.DeviceID)}
	propertyNames := []string{
		"object-identifier",
		"object-name",
		"vendor-name",
		"vendor-identifier",
		"model-name",
		"firmware-revision",
		"application-software-version",
		"location",
		"description",
		"protocol-version",
		"protocol-revision",
		"database-revision",
		"max-apdu",
		"segmentation-supported",
	}
	successes := 0
	for _, name := range propertyNames {
		property, parseErr := ParsePropertyIdentifier(name)
		if parseErr != nil {
			return identity, parseErr
		}
		value, readErr := c.readPropertyFromDevice(device, target.DeviceID, object, property, ^uint32(0))
		field := IdentityField{Property: property}
		if readErr != nil {
			field.Error = readErr.Error()
		} else {
			field.Value = value.Value
			field.ValueType = value.ValueType
			successes++
		}
		identity.Fields = append(identity.Fields, field)
	}
	if successes == 0 {
		return identity, fmt.Errorf("%w: device %d returned no identity properties", ErrRequest, target.DeviceID)
	}
	return identity, nil
}

func (c *nubeClient) Routers() ([]string, error) {
	addresses := c.raw.WhoIsRouterToNetwork()
	if addresses == nil {
		return nil, nil
	}
	out := make([]string, 0, len(*addresses))
	for _, address := range *addresses {
		udp, err := address.UDPAddr()
		if err == nil {
			out = append(out, fmt.Sprintf("%s network=%d", udp.String(), address.Net))
			continue
		}
		out = append(out, fmt.Sprintf("network=%d mac=%v", address.Net, address.Mac))
	}
	sort.Strings(out)
	return out, nil
}
