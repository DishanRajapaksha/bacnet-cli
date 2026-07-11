package bacnetclient

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	ErrValidation    = errors.New("validation error")
	ErrConnection    = errors.New("connection error")
	ErrRequest       = errors.New("request error")
	ErrWriteRejected = errors.New("write rejected")
)

type Options struct {
	Interface  string
	LocalIP    string
	Port       int
	SubnetCIDR int
	Timeout    time.Duration
}

type DiscoveryOptions struct {
	Low             int
	High            int
	GlobalBroadcast bool
	NetworkNumber   uint16
}

type Device struct {
	DeviceID      int    `json:"device_id"`
	Address       string `json:"address"`
	Port          int    `json:"port"`
	NetworkNumber int    `json:"network_number"`
	MSTPMAC       int    `json:"mstp_mac,omitempty"`
	MaxAPDU       uint32 `json:"max_apdu"`
	Segmentation  uint32 `json:"segmentation"`
	VendorID      uint32 `json:"vendor_id"`
}

type Target struct {
	DeviceID      int
	Address       string
	Port          int
	NetworkNumber int
	MSTPMAC       *int
	MaxAPDU       uint32
	Segmentation  uint32
}

type ObjectIdentifier struct {
	Type     uint16 `json:"type"`
	TypeName string `json:"type_name"`
	Instance uint32 `json:"instance"`
}

type PropertyIdentifier struct {
	ID   uint32 `json:"id"`
	Name string `json:"name"`
}

type PropertyValue struct {
	Timestamp  time.Time          `json:"timestamp"`
	Point      string             `json:"point,omitempty"`
	Unit       string             `json:"unit,omitempty"`
	DeviceID   int                `json:"device_id"`
	Object     ObjectIdentifier   `json:"object"`
	Property   PropertyIdentifier `json:"property"`
	ArrayIndex uint32             `json:"array_index"`
	Value      any                `json:"value"`
	ValueType  string             `json:"value_type"`
}

type Object struct {
	Type        uint16 `json:"type"`
	TypeName    string `json:"type_name"`
	Instance    uint32 `json:"instance"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

type WriteRequest struct {
	Target     Target
	Object     ObjectIdentifier
	Property   PropertyIdentifier
	ArrayIndex uint32
	Priority   uint8
	Value      any
}

type WritePlan struct {
	Point      string             `json:"point,omitempty"`
	Unit       string             `json:"unit,omitempty"`
	DeviceID   int                `json:"device_id"`
	Address    string             `json:"address,omitempty"`
	Object     ObjectIdentifier   `json:"object"`
	Property   PropertyIdentifier `json:"property"`
	ArrayIndex uint32             `json:"array_index"`
	Priority   uint8              `json:"priority,omitempty"`
	Value      any                `json:"value"`
	ValueType  string             `json:"value_type"`
	DryRun     bool               `json:"dry_run"`
}

type IdentityField struct {
	Property  PropertyIdentifier `json:"property"`
	Value     any                `json:"value,omitempty"`
	ValueType string             `json:"value_type,omitempty"`
	Error     string             `json:"error,omitempty"`
}

type DeviceIdentity struct {
	Timestamp time.Time       `json:"timestamp"`
	DeviceID  int             `json:"device_id"`
	Address   string          `json:"address,omitempty"`
	Fields    []IdentityField `json:"fields"`
}

type CatalogEntry struct {
	ID      uint32   `json:"id"`
	Name    string   `json:"name"`
	Aliases []string `json:"aliases,omitempty"`
}

var objectTypes = map[string]uint16{
	"analog-input": 0, "ai": 0,
	"analog-output": 1, "ao": 1,
	"analog-value": 2, "av": 2,
	"binary-input": 3, "bi": 3,
	"binary-output": 4, "bo": 4,
	"binary-value": 5, "bv": 5,
	"device":            8,
	"file":              10,
	"multi-state-input": 13, "msi": 13,
	"multi-state-output": 14, "mso": 14,
	"notification-class": 15,
	"multi-state-value":  19, "msv": 19,
	"trend-log":              20,
	"character-string-value": 40, "csv": 40,
}

var objectTypeNames = map[uint16]string{
	0: "analog-input", 1: "analog-output", 2: "analog-value",
	3: "binary-input", 4: "binary-output", 5: "binary-value",
	8: "device", 10: "file", 13: "multi-state-input",
	14: "multi-state-output", 15: "notification-class",
	19: "multi-state-value", 20: "trend-log", 40: "character-string-value",
}

var properties = map[string]uint32{
	"application-software-version":    12,
	"description":                     28,
	"firmware-revision":               44,
	"location":                        58,
	"max-apdu":                        62,
	"model-name":                      70,
	"object-identifier":               75,
	"object-list":                     76,
	"object-name":                     77,
	"object-type":                     79,
	"out-of-service":                  81,
	"present-value":                   85,
	"priority-array":                  87,
	"protocol-object-types-supported": 96,
	"protocol-services-supported":     97,
	"protocol-version":                98,
	"relinquish-default":              104,
	"segmentation-supported":          107,
	"status-flags":                    111,
	"system-status":                   112,
	"units":                           117,
	"vendor-identifier":               120,
	"vendor-name":                     121,
	"protocol-revision":               139,
	"database-revision":               155,
}

var propertyNames = func() map[uint32]string {
	out := make(map[uint32]string, len(properties))
	for name, id := range properties {
		out[id] = name
	}
	return out
}()

func ParseObjectIdentifier(value string) (ObjectIdentifier, error) {
	kind, instanceText, ok := strings.Cut(strings.TrimSpace(value), ":")
	if !ok || kind == "" || instanceText == "" {
		return ObjectIdentifier{}, fmt.Errorf("%w: object must be TYPE:INSTANCE, for example analog-input:1", ErrValidation)
	}
	var typeID uint64
	if parsed, err := strconv.ParseUint(kind, 10, 16); err == nil {
		typeID = parsed
	} else {
		id, found := objectTypes[strings.ToLower(kind)]
		if !found {
			return ObjectIdentifier{}, fmt.Errorf("%w: unsupported object type %q", ErrValidation, kind)
		}
		typeID = uint64(id)
	}
	instance, err := strconv.ParseUint(instanceText, 10, 22)
	if err != nil {
		return ObjectIdentifier{}, fmt.Errorf("%w: invalid object instance %q", ErrValidation, instanceText)
	}
	name := objectTypeNames[uint16(typeID)]
	if name == "" {
		name = fmt.Sprintf("object-%d", typeID)
	}
	return ObjectIdentifier{Type: uint16(typeID), TypeName: name, Instance: uint32(instance)}, nil
}

func ParsePropertyIdentifier(value string) (PropertyIdentifier, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return PropertyIdentifier{}, fmt.Errorf("%w: property is required", ErrValidation)
	}
	if id, err := strconv.ParseUint(value, 10, 22); err == nil {
		name := propertyNames[uint32(id)]
		if name == "" {
			name = fmt.Sprintf("property-%d", id)
		}
		return PropertyIdentifier{ID: uint32(id), Name: name}, nil
	}
	id, ok := properties[value]
	if !ok {
		return PropertyIdentifier{}, fmt.Errorf("%w: unsupported property %q; numeric property identifiers are accepted", ErrValidation, value)
	}
	return PropertyIdentifier{ID: id, Name: value}, nil
}

func ObjectTypeName(id uint16) string {
	if name := objectTypeNames[id]; name != "" {
		return name
	}
	return fmt.Sprintf("object-%d", id)
}

func ObjectTypeCatalog() []CatalogEntry {
	aliases := map[uint16][]string{}
	for alias, id := range objectTypes {
		if alias != objectTypeNames[id] {
			aliases[id] = append(aliases[id], alias)
		}
	}
	out := make([]CatalogEntry, 0, len(objectTypeNames))
	for id, name := range objectTypeNames {
		sort.Strings(aliases[id])
		out = append(out, CatalogEntry{ID: uint32(id), Name: name, Aliases: aliases[id]})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func PropertyCatalog() []CatalogEntry {
	out := make([]CatalogEntry, 0, len(propertyNames))
	for id, name := range propertyNames {
		out = append(out, CatalogEntry{ID: id, Name: name})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func ParseWriteValue(kind string, raw string, isNull bool) (any, error) {
	if isNull {
		return NullValue{}, nil
	}
	kind = strings.ToLower(strings.TrimSpace(kind))
	switch kind {
	case "string":
		return raw, nil
	case "bool", "boolean":
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid boolean value %q", ErrValidation, raw)
		}
		return value, nil
	case "uint", "unsigned", "enumerated":
		value, err := strconv.ParseUint(raw, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid unsigned value %q", ErrValidation, raw)
		}
		return uint32(value), nil
	case "int", "signed":
		value, err := strconv.ParseInt(raw, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid signed value %q", ErrValidation, raw)
		}
		return int32(value), nil
	case "float", "real", "float32":
		value, err := strconv.ParseFloat(raw, 32)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid float value %q", ErrValidation, raw)
		}
		return float32(value), nil
	case "double", "float64":
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid double value %q", ErrValidation, raw)
		}
		return value, nil
	case "":
		return nil, fmt.Errorf("%w: --type is required for writes unless --null is used", ErrValidation)
	default:
		return nil, fmt.Errorf("%w: write type must be string, bool, uint, int, float32, float64, or null", ErrValidation)
	}
}

type NullValue struct{}
