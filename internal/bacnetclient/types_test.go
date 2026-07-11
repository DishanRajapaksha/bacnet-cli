package bacnetclient

import "testing"

func TestParseObjectIdentifier(t *testing.T) {
	object, err := ParseObjectIdentifier("analog-input:12")
	if err != nil {
		t.Fatal(err)
	}
	if object.Type != 0 || object.Instance != 12 || object.TypeName != "analog-input" {
		t.Fatalf("unexpected object: %#v", object)
	}
}

func TestParseNumericObjectIdentifier(t *testing.T) {
	object, err := ParseObjectIdentifier("8:123")
	if err != nil {
		t.Fatal(err)
	}
	if object.Type != 8 || object.Instance != 123 {
		t.Fatalf("unexpected object: %#v", object)
	}
}

func TestParsePropertyIdentifier(t *testing.T) {
	property, err := ParsePropertyIdentifier("present-value")
	if err != nil {
		t.Fatal(err)
	}
	if property.ID != 85 {
		t.Fatalf("unexpected property: %#v", property)
	}
}

func TestParseWriteValue(t *testing.T) {
	value, err := ParseWriteValue("float32", "21.5", false)
	if err != nil {
		t.Fatal(err)
	}
	if value.(float32) != 21.5 {
		t.Fatalf("unexpected value: %#v", value)
	}
}
