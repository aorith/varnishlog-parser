package vsl

import (
	"testing"
)

func TestHeadersAddAndValues(t *testing.T) {
	headers := Headers{}

	// Add a single value
	headers.Add("x-test", "val1", HdrStateReceived)
	values := headers.Values("x-test")
	if len(values) != 1 {
		t.Fatalf("expected 1 value, got %d", len(values))
	}
	if values[0].Value() != "val1" || values[0].State() != HdrStateReceived {
		t.Errorf("unexpected value or state: %+v", values[0])
	}

	// Add a second value for the same header
	headers.Add("x-test", "val2", HdrStateAdded)
	values = headers.Values("x-test")
	if len(values) != 2 {
		t.Fatalf("expected 2 values, got %d", len(values))
	}
	if values[1].Value() != "val2" || values[1].State() != HdrStateAdded {
		t.Errorf("unexpected second value: %+v", values[1])
	}

	// Add a header with HdrStateModified (should replace existing values)
	headers.Add("x-test", "val3", HdrStateModified)
	values = headers.Values("x-test")
	if len(values) != 1 || values[0].Value() != "val3" || values[0].State() != HdrStateModified {
		t.Errorf("HdrStateModified did not replace previous values: %+v", values)
	}

	// Add Host header (should always have unique value)
	headers.Add("Host", "example.com", HdrStateAdded)
	headers.Add("Host", "other.com", HdrStateAdded)
	values = headers.Values(HdrNameHost)
	if len(values) != 1 || values[0].Value() != "other.com" {
		t.Errorf("Host header did not keep unique value: %+v", values)
	}

	value := headers.Get("host")
	if headers.Get("host") != "other.com" {
		t.Errorf("unexpected host header, got: %v, wanted: %v", value, "other.com")
	}
}

func TestHeadersDelete(t *testing.T) {
	headers := Headers{}
	headers.Add("X-Test", "value1", HdrStateReceived)
	headers.Add("X-Test", "value2", HdrStateAdded)

	headers.Delete("X-Test")
	values := headers.Values("X-Test")
	if len(values) != 2 {
		t.Fatalf("expected 2 values after delete, got %d", len(values))
	}

	for _, v := range values {
		if v.State() != HdrStateDeleted {
			t.Errorf("expected state Deleted, got %v", v.State())
		}
	}

	// Deleting a non-existent header should not panic
	headers.Delete("Non-Existent")
}

func TestCanonicalHeaderName(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"content-type", "Content-Type"},
		{"ACCEPT-encoding", "Accept-Encoding"},
		{"x-custom-header", "X-Custom-Header"},
	}

	for _, tt := range tests {
		got := CanonicalHeaderName(tt.input)
		if got != tt.want {
			t.Errorf("CanonicalHeaderName(%q) = %q; want %q", tt.input, got, tt.want)
		}
	}
}
