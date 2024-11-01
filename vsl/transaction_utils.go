package vsl

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

const (
	sizeByte SizeValue = 1
	sizeKB             = sizeByte * 1024
	SizeMB             = sizeKB * 1024
	sizeGB             = SizeMB * 1024
	sizeTB             = sizeGB * 1024
	sizePB             = sizeTB * 1024
)

// In Varnish the vxid is "uint32_t"
type VXID uint32

// SizeValue is a custom type based on int64 to handle sizes.
type SizeValue int64

// Value returns the size in bytes.
func (s SizeValue) Value() int64 {
	return int64(s)
}

// String returns the string representation of the size.
func (s SizeValue) String() string {
	switch {
	case s >= sizePB:
		return fmt.Sprintf("%.3fPB", float64(s)/float64(sizePB))
	case s >= sizeTB:
		return fmt.Sprintf("%.3fTB", float64(s)/float64(sizeTB))
	case s >= sizeGB:
		return fmt.Sprintf("%.3fGB", float64(s)/float64(sizeGB))
	case s >= SizeMB:
		return fmt.Sprintf("%.3fMB", float64(s)/float64(SizeMB))
	case s >= sizeKB:
		return fmt.Sprintf("%.3fKB", float64(s)/float64(sizeKB))
	default:
		return fmt.Sprintf("%dB", s)
	}
}

// convertStrToDuration converts a string to a duration value
func convertStrToDuration(s string, unit time.Duration) (time.Duration, error) {
	sfl, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return time.Duration(0), fmt.Errorf("Could not parse the string %q as a duration", s)
	}

	return time.Duration(sfl * float64(unit)), nil
}

// convertToUnixTimestamp converts a Unix timestamp string (integer or fractional) to a time.Time object
func convertToUnixTimestamp(s string) (time.Time, error) {
	unixnano, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("Timestamp with invalid float: %w", err)
	}

	seconds := int64(unixnano)
	// Varnish has microsecond precision, round it here to stick to that precision
	microseconds := int64(math.Round((unixnano - float64(seconds)) * 1e6))

	return time.Unix(seconds, microseconds*1e3), nil
}

// parseTXID returns an string that represents the transaction ID
func parseTXID(vxid VXID, recordType string, esiLevel int) string {
	if esiLevel > 0 {
		return fmt.Sprintf("%d_%s_esi_%d", vxid, recordType, esiLevel)
	}
	return fmt.Sprintf("%d_%s", vxid, recordType)
}

// parseLevel returns the level of the transaction parsing the initial transaction header
// e.g.  '**  << Request  >> 2'
// it checks the first field of the line ('*', '**', '*3*' ...)
func parseLevel(s string) (int, error) {
	stars := strings.Count(s, "*")
	if stars == len(s) {
		return stars, nil
	}

	// If we are here, the string must be something like '*5*'
	var sb strings.Builder
	for _, r := range s {
		if r != '*' {
			sb.WriteRune(r)
		}
	}

	levelStr := sb.String()
	level, err := strconv.Atoi(levelStr)
	if err != nil {
		return 0, err
	}
	return level, nil
}

// parseVXID parses an string as a VXID
func parseVXID(s string) (VXID, error) {
	vxid, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return VXID(0), fmt.Errorf("VXID parse failed, error: %s", err)
	}
	return VXID(vxid), nil
}

// collectAllChildren is a helper function to recursively collect all children and their descendants
func collectAllChildren(parent *Transaction) []*Transaction {
	visited := make(map[string]bool)

	var recursiveCollect func(tx *Transaction) []*Transaction
	recursiveCollect = func(tx *Transaction) []*Transaction {
		var allChildren []*Transaction

		if visited[tx.TXID()] {
			return allChildren
		}
		visited[tx.TXID()] = true

		children := tx.ChildrenSortedByVXID()
		for _, child := range children {
			allChildren = append(allChildren, child)
			allChildren = append(allChildren, recursiveCollect(child)...)
		}

		return allChildren
	}

	return recursiveCollect(parent)
}
