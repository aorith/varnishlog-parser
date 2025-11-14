// SPDX-License-Identifier: MIT

package render

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/aorith/varnishlog-parser/vsl"
)

// ParseBackend parses "<HOST/IP>:<PORT>" where HOST may be a hostname,
//
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

	// If it is an IPv6, surround it with []
	if strings.Contains(host, ":") && !strings.Contains(host, "[") && !strings.Contains(host, "]") {
		host = fmt.Sprintf("[%s]", host)
	}

	return host, port, nil
}

// ttlRecordHTML returns a TTL record in HTML format using abbr for human readable dates
func ttlRecordHTML(r vsl.TTLRecord) string {
	if r.Source == "RFC" {
		return fmt.Sprintf(
			`%s | TTL %s, Grace %s, Keep %s, Reference <abbr title="%s">%d</abbr>, Age <abbr title="%s">%d</abbr>, Date <abbr title="%s">%d</abbr>, Expires <abbr title="%s">%d</abbr>, Max-Age %s | %s`,
			r.Source,
			r.TTL.String(),
			r.Grace.String(),
			r.Keep.String(),
			r.Reference.String(),
			r.Reference.Unix(),
			r.Age.String(),
			r.Age.Unix(),
			r.Date.String(),
			r.Date.Unix(),
			r.Expires.String(),
			r.Expires.Unix(),
			r.MaxAge.String(),
			r.CacheStatus,
		)
	}

	return fmt.Sprintf(
		`%s | TTL %s, Grace %s, Keep %s, Reference <abbr title="%s">%d</abbr> | %s`,
		r.Source,
		r.TTL.String(),
		r.Grace.String(),
		r.Keep.String(),
		r.Reference.String(),
		r.Reference.Unix(),
		r.CacheStatus,
	)
}

// timestampRecordHTML returns a Timestamp record in HTML format using abbr for human readable dates
func timestampRecordHTML(r vsl.TimestampRecord) string {
	absTime := float64(r.AbsoluteTime.UnixMicro()) / 1e6
	return fmt.Sprintf(
		`%s | Elapsed: %s | Total: %s | <abbr title="%s">%.6f</abbr>`,
		r.EventLabel, r.SinceLast.String(), r.SinceStart.String(), r.AbsoluteTime.String(), absTime,
	)
}
