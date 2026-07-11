package cli

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/DishanRajapaksha/bacnet-cli/internal/bacnetclient"
	"github.com/DishanRajapaksha/bacnet-cli/internal/config"
	"github.com/DishanRajapaksha/bacnet-cli/internal/devicemap"
	"github.com/DishanRajapaksha/bacnet-cli/internal/output"
)

func renderConfiguredDevices(w io.Writer, format string, devices []config.DeviceConfig) error {
	format = output.NormaliseFormat(format)
	if err := output.ValidateSnapshotFormat(format); err != nil {
		return err
	}
	if format == output.FormatJSON {
		return output.WriteJSON(w, devices)
	}
	rows := make([][]string, 0, len(devices))
	for _, device := range devices {
		target := devicemap.ResolveDevice(device)
		mstp := ""
		if target.MSTPMAC != nil {
			mstp = strconv.Itoa(*target.MSTPMAC)
		}
		rows = append(rows, []string{
			device.Name,
			strconv.Itoa(target.DeviceID),
			target.Address,
			strconv.Itoa(target.Port),
			strconv.Itoa(target.NetworkNumber),
			mstp,
			strconv.FormatUint(uint64(target.MaxAPDU), 10),
			strconv.FormatUint(uint64(target.Segmentation), 10),
		})
	}
	headers := []string{"NAME", "DEVICE_ID", "ADDRESS", "PORT", "NETWORK", "MSTP", "MAX_APDU", "SEGMENTATION"}
	return renderRows(w, format, headers, rows)
}

func renderConfiguredPoints(w io.Writer, format string, points []config.PointConfig) error {
	format = output.NormaliseFormat(format)
	if err := output.ValidateSnapshotFormat(format); err != nil {
		return err
	}
	if format == output.FormatJSON {
		return output.WriteJSON(w, points)
	}
	rows := make([][]string, 0, len(points))
	for _, point := range points {
		property := point.Property
		if property == "" {
			property = "present-value"
		}
		arrayIndex := "all"
		if point.ArrayIndex != nil {
			arrayIndex = strconv.FormatUint(uint64(*point.ArrayIndex), 10)
		}
		priority := ""
		if point.Writable {
			value := point.Priority
			if value == 0 {
				value = 16
			}
			priority = strconv.Itoa(int(value))
		}
		rows = append(rows, []string{
			point.Name, point.Device, point.Object, property, arrayIndex,
			point.Type, point.Unit, strconv.FormatBool(point.Writable), priority,
		})
	}
	headers := []string{"NAME", "DEVICE", "OBJECT", "PROPERTY", "INDEX", "TYPE", "UNIT", "WRITABLE", "PRIORITY"}
	return renderRows(w, format, headers, rows)
}

func renderPointValue(w io.Writer, format string, value bacnetclient.PropertyValue) error {
	format = output.NormaliseFormat(format)
	if err := output.ValidateSnapshotFormat(format); err != nil {
		return err
	}
	if format == output.FormatJSON {
		return output.WriteJSON(w, value)
	}
	row := []string{
		value.Timestamp.Format("2006-01-02T15:04:05.000Z07:00"),
		value.Point,
		strconv.Itoa(value.DeviceID),
		fmt.Sprintf("%s:%d", value.Object.TypeName, value.Object.Instance),
		fmt.Sprintf("%s(%d)", value.Property.Name, value.Property.ID),
		formatValue(value.Value),
		value.Unit,
		value.ValueType,
	}
	headers := []string{"TIMESTAMP", "POINT", "DEVICE", "OBJECT", "PROPERTY", "VALUE", "UNIT", "TYPE"}
	return renderRows(w, format, headers, [][]string{row})
}

func renderPointWritePlan(w io.Writer, format string, plan bacnetclient.WritePlan) error {
	format = output.NormaliseFormat(format)
	if err := output.ValidateSnapshotFormat(format); err != nil {
		return err
	}
	if format == output.FormatJSON {
		return output.WriteJSON(w, plan)
	}
	row := []string{
		plan.Point,
		strconv.Itoa(plan.DeviceID),
		plan.Address,
		fmt.Sprintf("%s:%d", plan.Object.TypeName, plan.Object.Instance),
		plan.Property.Name,
		formatValue(plan.Value),
		plan.Unit,
		plan.ValueType,
		strconv.Itoa(int(plan.Priority)),
		strconv.FormatBool(plan.DryRun),
	}
	headers := []string{"POINT", "DEVICE", "ADDRESS", "OBJECT", "PROPERTY", "VALUE", "UNIT", "TYPE", "PRIORITY", "DRY_RUN"}
	return renderRows(w, format, headers, [][]string{row})
}

func renderPointStream(w io.Writer, format string, value bacnetclient.PropertyValue, csvHeader bool) error {
	format = output.NormaliseFormat(format)
	if err := output.ValidateStreamFormat(format); err != nil {
		return err
	}
	if format == output.FormatJSONL {
		return output.WriteJSONLine(w, value)
	}
	row := []string{
		value.Timestamp.Format("2006-01-02T15:04:05.000Z07:00"),
		value.Point,
		strconv.Itoa(value.DeviceID),
		fmt.Sprintf("%s:%d", value.Object.TypeName, value.Object.Instance),
		value.Property.Name,
		formatValue(value.Value),
		value.Unit,
		value.ValueType,
	}
	if format == output.FormatCSV {
		headers := []string(nil)
		if csvHeader {
			headers = []string{"timestamp", "point", "device", "object", "property", "value", "unit", "type"}
		}
		return output.WriteCSV(w, headers, [][]string{row})
	}
	return output.WriteText(w, strings.Join(row, " "))
}

func renderIdentity(w io.Writer, format string, identity bacnetclient.DeviceIdentity) error {
	format = output.NormaliseFormat(format)
	if err := output.ValidateSnapshotFormat(format); err != nil {
		return err
	}
	if format == output.FormatJSON {
		return output.WriteJSON(w, identity)
	}
	rows := make([][]string, 0, len(identity.Fields))
	for _, field := range identity.Fields {
		rows = append(rows, []string{
			strconv.FormatUint(uint64(field.Property.ID), 10),
			field.Property.Name,
			formatValue(field.Value),
			field.ValueType,
			field.Error,
		})
	}
	headers := []string{"ID", "PROPERTY", "VALUE", "TYPE", "ERROR"}
	return renderRows(w, format, headers, rows)
}

func renderCatalog(w io.Writer, format string, entries []bacnetclient.CatalogEntry) error {
	format = output.NormaliseFormat(format)
	if err := output.ValidateSnapshotFormat(format); err != nil {
		return err
	}
	if format == output.FormatJSON {
		return output.WriteJSON(w, entries)
	}
	rows := make([][]string, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, []string{
			strconv.FormatUint(uint64(entry.ID), 10),
			entry.Name,
			strings.Join(entry.Aliases, ","),
		})
	}
	headers := []string{"ID", "NAME", "ALIASES"}
	return renderRows(w, format, headers, rows)
}

func renderRows(w io.Writer, format string, headers []string, rows [][]string) error {
	switch format {
	case output.FormatCSV:
		return output.WriteCSV(w, headers, rows)
	case output.FormatText:
		for _, row := range rows {
			if err := output.WriteText(w, strings.Join(row, " ")); err != nil {
				return err
			}
		}
		return nil
	default:
		return output.WriteTable(w, headers, rows)
	}
}
