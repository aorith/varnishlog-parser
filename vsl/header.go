package vsl

import (
	"net/textproto"
)

// The RFCs allow multiple headers with the same name, and both set and unset
// within VCL will remove all headers with the name given.

// Special headers
var (
	HdrNameHost = CanonicalHeaderName("Host")
)

// HdrState represents the state of an HTTP header within the Varnish lifecycle.
// It indicates whether a header was originally received, added, modified, or deleted.
type HdrState int

const (
	HdrStateReceived HdrState = iota // Header as sent by the client
	HdrStateAdded                    // Header added within VCL
	HdrStateModified                 // Header modified within VCL
	HdrStateDeleted                  // Header deleted within VCL
)

// String returns a human-readable representation of the HdrState.
func (s HdrState) String() string {
	switch s {
	case HdrStateReceived:
		return "Received"
	case HdrStateAdded:
		return "Added"
	case HdrStateModified:
		return "Modified"
	case HdrStateDeleted:
		return "Deleted"
	default:
		panic("vsl: unknown HdrState")
	}
}

// Header represents an HTTP header within the VSL
type Header struct {
	name           string
	values         []HdrValue // Keeps track of the headers after VCL code execution
	receivedValues []HdrValue // Keeps track of the headers that were sent by the client
}

func (h Header) Name() string {
	return h.name
}

// Values returns all the values
// When received is true, it returns the receivedValues
func (h Header) Values(received bool) []HdrValue {
	if received {
		return h.receivedValues
	}
	return h.values
}

// HdrValue represents a single header value and its state
type HdrValue struct {
	value string
	state HdrState
}

// Value returns the header value
func (h HdrValue) Value() string {
	return h.value
}

// State returns the header state
func (h HdrValue) State() HdrState {
	return h.state
}

// Headers represents a set of HTTP headers within the VSL
type Headers map[string]Header

// Add adds a header value to the Headers map.
// If the header already exists, the value is appended.
// Otherwise, a new Header is created.
// If the state is 'modified', previous values are discarded
// as Varnish VCL removes all the previous values on 'set' and 'unset'.
func (h Headers) Add(name string, value string, state HdrState) {
	name = CanonicalHeaderName(name)

	// Check if header already exists
	header, exists := h[name]
	if !exists {
		header = Header{
			name:           name,
			values:         []HdrValue{},
			receivedValues: []HdrValue{},
		}
	}

	// If the state is modified, delete all the previous values.
	// A check must be done before calling Add to determine
	// if the modified state should be used
	//
	// If the header name is Host, it also accepts an unique value
	if state == HdrStateModified || name == HdrNameHost {
		header.values = []HdrValue{}
	}

	header.values = append(header.values, HdrValue{
		value: value,
		state: state,
	})

	// If the state is received, append it to the received slice
	if state == HdrStateReceived {
		header.receivedValues = append(header.receivedValues, HdrValue{
			value: value,
			state: state,
		})
	}

	h[name] = header
}

// Delete sets the state of all values of the given header as deleted.
func (h Headers) Delete(name string) {
	name = CanonicalHeaderName(name)

	header, exists := h[name]
	if !exists {
		// Nothing to delete
		return
	}

	// Mark all values as deleted
	for i := range header.values {
		header.values[i].state = HdrStateDeleted
	}
	for i := range header.receivedValues {
		header.receivedValues[i].state = HdrStateDeleted
	}

	h[name] = header
}

// Values returns all the values associated with the given header name
// When received is true, it returns the values from the receivedValues slice.
func (h Headers) Values(name string, received bool) []HdrValue {
	name = CanonicalHeaderName(name)
	header, exists := h[name]
	if !exists {
		return nil
	}
	if received {
		return header.receivedValues
	}
	return header.values
}

// Get gets the first value associated with the given header name.
// When received is true it only returns values from the receivedValues slice.
// If there are no values it returns ""
func (h Headers) Get(name string, received bool) string {
	name = CanonicalHeaderName(name)
	header, exists := h[name]
	if !exists {
		return ""
	}
	var values []HdrValue
	if received {
		values = header.receivedValues
	} else {
		values = header.values
	}
	if len(values) == 0 {
		return ""
	}
	return values[0].Value()
}

// CanonicalHeaderName returns the canonical format of the
// header name. The canonicalization converts the first
// letter and any letter following a hyphen to upper case;
// the rest are converted to lowercase. For example, the
// canonical key for "accept-encoding" is "Accept-Encoding".
func CanonicalHeaderName(name string) string {
	return textproto.CanonicalMIMEHeaderKey(name)
}
