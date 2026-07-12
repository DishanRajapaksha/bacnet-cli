package output

import (
	"io"

	shared "github.com/DishanRajapaksha/industrial-cli-kit/output"
)

var ErrOutput = shared.ErrOutput

const (
	FormatTable = shared.FormatTable
	FormatText  = shared.FormatText
	FormatJSON  = shared.FormatJSON
	FormatJSONL = shared.FormatJSONL
	FormatCSV   = shared.FormatCSV
)

func NormaliseFormat(value string) string        { return shared.NormaliseFormat(value) }
func ValidateSnapshotFormat(value string) error  { return shared.ValidateSnapshotFormat(value) }
func ValidateStreamFormat(value string) error    { return shared.ValidateStreamFormat(value) }
func WriteJSON(w io.Writer, value any) error     { return shared.WriteJSON(w, value) }
func WriteJSONLine(w io.Writer, value any) error { return shared.WriteJSONLine(w, value) }
func WriteText(w io.Writer, value any) error     { return shared.WriteText(w, value) }
func WriteTable(w io.Writer, headers []string, rows [][]string) error {
	return shared.WriteTable(w, headers, rows)
}
func WriteCSV(w io.Writer, headers []string, rows [][]string) error {
	return shared.WriteCSV(w, headers, rows)
}
