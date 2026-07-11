package output

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

var ErrOutput = errors.New("output error")

const (
	FormatTable = "table"
	FormatText  = "text"
	FormatJSON  = "json"
	FormatJSONL = "jsonl"
	FormatCSV   = "csv"
)

func NormaliseFormat(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", FormatTable:
		return FormatTable
	case FormatText:
		return FormatText
	case FormatJSON:
		return FormatJSON
	case FormatJSONL:
		return FormatJSONL
	case FormatCSV:
		return FormatCSV
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func ValidateSnapshotFormat(value string) error {
	switch NormaliseFormat(value) {
	case FormatTable, FormatText, FormatJSON, FormatCSV:
		return nil
	default:
		return fmt.Errorf("%w: snapshot format must be table, text, json, or csv", ErrOutput)
	}
}

func ValidateStreamFormat(value string) error {
	switch NormaliseFormat(value) {
	case FormatText, FormatJSONL, FormatCSV:
		return nil
	default:
		return fmt.Errorf("%w: stream format must be text, jsonl, or csv", ErrOutput)
	}
}

func WriteJSON(w io.Writer, value any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(value); err != nil {
		return fmt.Errorf("%w: %v", ErrOutput, err)
	}
	return nil
}

func WriteJSONLine(w io.Writer, value any) error {
	if err := json.NewEncoder(w).Encode(value); err != nil {
		return fmt.Errorf("%w: %v", ErrOutput, err)
	}
	return nil
}

func WriteText(w io.Writer, value any) error {
	if _, err := fmt.Fprintln(w, value); err != nil {
		return fmt.Errorf("%w: %v", ErrOutput, err)
	}
	return nil
}

func WriteTable(w io.Writer, headers []string, rows [][]string) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, strings.Join(headers, "\t")); err != nil {
		return fmt.Errorf("%w: %v", ErrOutput, err)
	}
	for _, row := range rows {
		if _, err := fmt.Fprintln(tw, strings.Join(row, "\t")); err != nil {
			return fmt.Errorf("%w: %v", ErrOutput, err)
		}
	}
	if err := tw.Flush(); err != nil {
		return fmt.Errorf("%w: %v", ErrOutput, err)
	}
	return nil
}

func WriteCSV(w io.Writer, headers []string, rows [][]string) error {
	cw := csv.NewWriter(w)
	if len(headers) > 0 {
		if err := cw.Write(headers); err != nil {
			return fmt.Errorf("%w: %v", ErrOutput, err)
		}
	}
	for _, row := range rows {
		if err := cw.Write(row); err != nil {
			return fmt.Errorf("%w: %v", ErrOutput, err)
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return fmt.Errorf("%w: %v", ErrOutput, err)
	}
	return nil
}
