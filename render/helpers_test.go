// SPDX-License-Identifier: MIT

package render_test

import (
	"testing"

	"github.com/aorith/varnishlog-parser/render"
)

func TestParseBackend(t *testing.T) {
	tests := []struct {
		in       string
		wantHost string
		wantPort string
		wantErr  bool
	}{
		// Host only, no port - curl's --connect-to treats an empty PORT2 as
		// "keep the request's original port" (e.g. --connect-to '::10.0.0.1:'
		// to override just the host across both http/https).
		{"10.0.0.1", "10.0.0.1", "", false},
		{"example.com", "example.com", "", false},
		{"::1", "[::1]", "", false},
		{"fe80::1", "[fe80::1]", "", false},
		{"2001:db8::1", "[2001:db8::1]", "", false},
		{"[::1]", "[::1]", "", false},
		{"[::1]:", "[::1]", "", false},

		// Host and port.
		{"10.0.0.1:8080", "10.0.0.1", "8080", false},
		{"example.com:8080", "example.com", "8080", false},
		{"[::1]:8080", "[::1]", "8080", false},

		// Errors.
		{"fe80::1%eth0", "", "", true}, // unbracketed zoned IPv6 with no port is ambiguous
	}

	for _, tt := range tests {
		host, port, err := render.ParseBackend(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParseBackend(%q): expected an error, got host=%q port=%q", tt.in, host, port)
			}

			continue
		}

		if err != nil {
			t.Errorf("ParseBackend(%q) failed: %s", tt.in, err)

			continue
		}

		if host != tt.wantHost || port != tt.wantPort {
			t.Errorf("ParseBackend(%q) = host=%q port=%q, want host=%q port=%q", tt.in, host, port, tt.wantHost, tt.wantPort)
		}
	}
}
