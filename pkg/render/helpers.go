package render

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

type CustomBuilder struct {
	strings.Builder
}

// PadAdd is a helper function to append a fixed padding to the string
func (b *CustomBuilder) PadAdd(s string) {
	b.WriteString("    " + s + "\n")
}

// ParseBackend parses "<HOST/IP>:<PORT>" where HOST may be a hostname,
// IPv4, or IPv6 (possibly unbracketed). Returns host and port separately.
func ParseBackend(s string) (host, port string, err error) {
	// Try the standard parser first (works for "host:port" and "[v6]:port").
	host, port, err = net.SplitHostPort(s)
	if err != nil {
		// Fallback: split at the last colon. This handles unbracketed IPv6 like "fe80::1%eth0:8080".
		i := strings.LastIndex(s, ":")
		if i == -1 {
			return "", "", fmt.Errorf("missing port in %q", s)
		}
		host = s[:i]
		port = s[i+1:]
		if host == "" {
			return "", "", fmt.Errorf("empty host in %q", s)
		}
		if _, e := strconv.Atoi(port); e != nil {
			return "", "", fmt.Errorf("invalid port %q: %v", port, e)
		}
	}

	// If it is an IPv6, surrond it with []
	if strings.Contains(host, ":") && !strings.Contains(host, "[") && !strings.Contains(host, "]") {
		host = fmt.Sprintf("[%s]", host)
	}

	return host, port, nil
}
