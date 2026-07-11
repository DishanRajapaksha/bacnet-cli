package cli

import (
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"

	"github.com/DishanRajapaksha/bacnet-cli/internal/bacnetclient"
	"github.com/DishanRajapaksha/bacnet-cli/internal/output"
)

func renderDevices(w io.Writer, format string, devices []bacnetclient.Device) error {
	format = output.NormaliseFormat(format)
	if err := output.ValidateSnapshotFormat(format); err != nil {
		return err
	}
	if format == output.FormatJSON {
		return output.WriteJSON(w, devices)
	}
	rows := make([][]string, 0, len(devices))
	for _, device := range devices {
		rows = append(rows, []string{
			strconv.Itoa(device.DeviceID), device.Address, strconv.Itoa(device.Port),
			strconv.Itoa(device.NetworkNumber), strconv.Itoa(device.MSTPMAC),
			strconv.FormatUint(uint64(device.VendorID), 10), strconv.FormatUint(uint64(device.MaxAPDU), 10),
		})
	}
	headers := []string{"DEVICE", "ADDRESS", "PORT", "NETWORK", "MSTP", "VENDOR", "MAX_APDU"}
	if format == output.FormatCSV {
		return output.WriteCSV(w, headers, rows)
	}
	if format == output.FormatText {
		for _, row := range rows {
			if err := output.WriteText(w, strings.Join(row, " ")); err != nil {
				return err
			}
		}
		return nil
	}
	return output.WriteTable(w, headers, rows)
}

func renderProperty(w io.Writer, format string, value bacnetclient.PropertyValue) error {
	format = output.NormaliseFormat(format)
	if err := output.ValidateSnapshotFormat(format); err != nil {
		return err
	}
	if format == output.FormatJSON {
		return output.WriteJSON(w, value)
	}
	row := []string{
		value.Timestamp.Format("2006-01-02T15:04:05.000Z07:00"),
		strconv.Itoa(value.DeviceID),
		fmt.Sprintf("%s:%d", value.Object.TypeName, value.Object.Instance),
		fmt.Sprintf("%s(%d)", value.Property.Name, value.Property.ID),
		strconv.FormatUint(uint64(value.ArrayIndex), 10),
		formatValue(value.Value),
		value.ValueType,
	}
	headers := []string{"TIMESTAMP", "DEVICE", "OBJECT", "PROPERTY", "INDEX", "VALUE", "TYPE"}
	if format == output.FormatCSV {
		return output.WriteCSV(w, headers, [][]string{row})
	}
	if format == output.FormatText {
		return output.WriteText(w, fmt.Sprintf("%s %s=%s", row[2], row[3], row[5]))
	}
	return output.WriteTable(w, headers, [][]string{row})
}

func renderPropertyStream(w io.Writer, format string, value bacnetclient.PropertyValue, csvHeader bool) error {
	format = output.NormaliseFormat(format)
	if err := output.ValidateStreamFormat(format); err != nil {
		return err
	}
	if format == output.FormatJSONL {
		return output.WriteJSONLine(w, value)
	}
	row := []string{
		value.Timestamp.Format("2006-01-02T15:04:05.000Z07:00"),
		strconv.Itoa(value.DeviceID),
		fmt.Sprintf("%s:%d", value.Object.TypeName, value.Object.Instance),
		value.Property.Name,
		formatValue(value.Value),
		value.ValueType,
	}
	if format == output.FormatCSV {
		headers := []string(nil)
		if csvHeader {
			headers = []string{"timestamp", "device", "object", "property", "value", "type"}
		}
		return output.WriteCSV(w, headers, [][]string{row})
	}
	return output.WriteText(w, strings.Join(row, " "))
}

func renderObjects(w io.Writer, format string, objects []bacnetclient.Object) error {
	format = output.NormaliseFormat(format)
	if err := output.ValidateSnapshotFormat(format); err != nil {
		return err
	}
	if format == output.FormatJSON {
		return output.WriteJSON(w, objects)
	}
	rows := make([][]string, 0, len(objects))
	for _, object := range objects {
		rows = append(rows, []string{
			strconv.FormatUint(uint64(object.Type), 10), object.TypeName,
			strconv.FormatUint(uint64(object.Instance), 10), object.Name, object.Description,
		})
	}
	headers := []string{"TYPE", "TYPE_NAME", "INSTANCE", "NAME", "DESCRIPTION"}
	if format == output.FormatCSV {
		return output.WriteCSV(w, headers, rows)
	}
	if format == output.FormatText {
		for _, row := range rows {
			if err := output.WriteText(w, fmt.Sprintf("%s:%s %s %s", row[1], row[2], row[3], row[4])); err != nil {
				return err
			}
		}
		return nil
	}
	return output.WriteTable(w, headers, rows)
}

func renderWritePlan(w io.Writer, format string, plan bacnetclient.WritePlan) error {
	format = output.NormaliseFormat(format)
	if err := output.ValidateSnapshotFormat(format); err != nil {
		return err
	}
	if format == output.FormatJSON {
		return output.WriteJSON(w, plan)
	}
	row := []string{
		strconv.Itoa(plan.DeviceID), plan.Address,
		fmt.Sprintf("%s:%d", plan.Object.TypeName, plan.Object.Instance),
		plan.Property.Name, formatValue(plan.Value), plan.ValueType,
		strconv.Itoa(int(plan.Priority)), strconv.FormatBool(plan.DryRun),
	}
	headers := []string{"DEVICE", "ADDRESS", "OBJECT", "PROPERTY", "VALUE", "TYPE", "PRIORITY", "DRY_RUN"}
	if format == output.FormatCSV {
		return output.WriteCSV(w, headers, [][]string{row})
	}
	if format == output.FormatText {
		return output.WriteText(w, fmt.Sprintf("device=%s object=%s property=%s value=%s dry_run=%s", row[0], row[2], row[3], row[4], row[7]))
	}
	return output.WriteTable(w, headers, [][]string{row})
}

func formatValue(value any) string {
	if value == nil {
		return "null"
	}
	if _, ok := value.(bacnetclient.NullValue); ok {
		return "null"
	}
	if stringer, ok := value.(fmt.Stringer); ok {
		return stringer.String()
	}
	rv := reflect.ValueOf(value)
	if rv.Kind() == reflect.Ptr && !rv.IsNil() {
		return formatValue(rv.Elem().Interface())
	}
	return fmt.Sprint(value)
}
