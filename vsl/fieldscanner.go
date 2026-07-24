// SPDX-License-Identifier: MIT

package vsl

import (
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"
	"time"
)

// fieldScanner extracts positional, whitespace-separated fields out of a VSL record's
// raw value. Every extraction method is a no-op once an error has been recorded, so a
// constructor can pull every field unconditionally and check err() once at the end
// instead of after each individual field.
type fieldScanner struct {
	blr    BaseRecord
	target string // Go type name used in error messages, e.g. "HitRecord"
	parts  []string
	fail   error
}

// newFieldScanner splits the record's raw value on whitespace.
func newFieldScanner(blr BaseRecord, target string) *fieldScanner {
	return &fieldScanner{blr: blr, target: target, parts: strings.Fields(blr.GetRawValue())}
}

// err returns the first error encountered, if any.
func (f *fieldScanner) err() error {
	return f.fail
}

// count returns the number of whitespace-separated fields found.
func (f *fieldScanner) count() int {
	return len(f.parts)
}

// require fails unless the field count is one of the accepted values.
func (f *fieldScanner) require(accepted ...int) {
	if f.fail != nil {
		return
	}

	if !slices.Contains(accepted, len(f.parts)) {
		f.fail = fmt.Errorf("conversion to %s failed, incorrect len of %d on line %q", f.target, len(f.parts), f.blr.GetRawLog())
	}
}

// requireMin fails if there are fewer than n fields.
func (f *fieldScanner) requireMin(n int) {
	if f.fail != nil {
		return
	}

	if len(f.parts) < n {
		f.fail = fmt.Errorf("conversion to %s failed, incorrect len of %d on line %q", f.target, len(f.parts), f.blr.GetRawLog())
	}
}

// str returns field i, failing if it isn't present.
func (f *fieldScanner) str(name string, i int) string {
	if f.fail != nil {
		return ""
	}

	if i >= len(f.parts) {
		f.fail = fmt.Errorf("conversion to %s failed, missing field %s on line %q", f.target, name, f.blr.GetRawLog())

		return ""
	}

	return f.parts[i]
}

// strOr returns field i, or def if it isn't present. Never fails.
func (f *fieldScanner) strOr(i int, def string) string {
	if f.fail != nil || i >= len(f.parts) {
		return def
	}

	return f.parts[i]
}

// literal fails unless field i equals want.
func (f *fieldScanner) literal(name string, i int, want string) {
	s := f.str(name, i)
	if f.fail != nil || s == want {
		return
	}

	f.fail = fmt.Errorf("conversion to %s failed, expected field %s to be %q on line %q", f.target, name, want, f.blr.GetRawLog())
}

// rune returns field i as a single rune, failing unless it is exactly one
// character long and is one of allowed.
func (f *fieldScanner) rune(name string, i int, allowed ...rune) rune {
	s := f.str(name, i)
	if f.fail != nil {
		return 0
	}

	if len(s) != 1 || !slices.Contains(allowed, rune(s[0])) {
		f.fail = fmt.Errorf("conversion to %s failed, bad field %s on line %q", f.target, name, f.blr.GetRawLog())

		return 0
	}

	return rune(s[0])
}

// oneOf returns field i, failing unless it is one of allowed.
func (f *fieldScanner) oneOf(name string, i int, allowed ...string) string {
	s := f.str(name, i)
	if f.fail != nil {
		return ""
	}

	if !slices.Contains(allowed, s) {
		f.fail = fmt.Errorf("conversion to %s failed, bad field %s on line %q", f.target, name, f.blr.GetRawLog())

		return ""
	}

	return s
}

// int parses field i as a base-10 integer.
func (f *fieldScanner) int(name string, i int) int {
	s := f.str(name, i)
	if f.fail != nil {
		return 0
	}

	v, err := strconv.Atoi(s)
	if err != nil {
		f.fail = fmt.Errorf("conversion to %s failed, bad field %s on line %q", f.target, name, f.blr.GetRawLog())

		return 0
	}

	return v
}

// int64 parses field i as a base-10, 64-bit integer.
func (f *fieldScanner) int64(name string, i int) int64 {
	s := f.str(name, i)
	if f.fail != nil {
		return 0
	}

	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		f.fail = fmt.Errorf("conversion to %s failed, bad field %s on line %q", f.target, name, f.blr.GetRawLog())

		return 0
	}

	return v
}

// size parses field i as a byte count.
func (f *fieldScanner) size(name string, i int) SizeValue {
	return SizeValue(f.int(name, i))
}

// duration parses field i as a floating point number of unit.
func (f *fieldScanner) duration(name string, i int, unit time.Duration) time.Duration {
	s := f.str(name, i)
	if f.fail != nil {
		return 0
	}

	d, err := convertStrToDuration(s, unit)
	if err != nil {
		f.fail = fmt.Errorf("conversion to %s failed, bad field %s on line %q", f.target, name, f.blr.GetRawLog())

		return 0
	}

	return d
}

// unixTime parses field i as a Unix timestamp (integer or fractional seconds).
func (f *fieldScanner) unixTime(name string, i int) time.Time {
	s := f.str(name, i)
	if f.fail != nil {
		return time.Time{}
	}

	t, err := convertToUnixTimestamp(s)
	if err != nil {
		f.fail = fmt.Errorf("conversion to %s failed, bad field %s on line %q", f.target, name, f.blr.GetRawLog())

		return time.Time{}
	}

	return t
}

// ip parses field i as an IPv4/IPv6 address, trimming surrounding brackets.
func (f *fieldScanner) ip(name string, i int) net.IP {
	s := f.str(name, i)
	if f.fail != nil {
		return nil
	}

	ip := net.ParseIP(strings.Trim(s, "[]"))
	if ip == nil {
		f.fail = fmt.Errorf("conversion to %s failed, bad field %s on line %q", f.target, name, f.blr.GetRawLog())

		return nil
	}

	return ip
}

// vxid parses field i as a VXID.
func (f *fieldScanner) vxid(name string, i int) VXID {
	s := f.str(name, i)
	if f.fail != nil {
		return 0
	}

	v, err := parseVXID(s)
	if err != nil {
		f.fail = fmt.Errorf("conversion to %s failed, bad field %s on line %q", f.target, name, f.blr.GetRawLog())

		return 0
	}

	return v
}
