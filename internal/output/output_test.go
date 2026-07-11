package output

import "testing"

func TestSnapshotFormatContract(t *testing.T) {
	for _, format := range []string{"table", "text", "json", "csv"} {
		if err := ValidateSnapshotFormat(format); err != nil {
			t.Fatalf("%s: %v", format, err)
		}
	}
	if err := ValidateSnapshotFormat("jsonl"); err == nil {
		t.Fatal("jsonl must not be accepted for snapshots")
	}
}

func TestStreamFormatContract(t *testing.T) {
	for _, format := range []string{"text", "jsonl", "csv"} {
		if err := ValidateStreamFormat(format); err != nil {
			t.Fatalf("%s: %v", format, err)
		}
	}
	if err := ValidateStreamFormat("json"); err == nil {
		t.Fatal("json must not be accepted for streams")
	}
}
