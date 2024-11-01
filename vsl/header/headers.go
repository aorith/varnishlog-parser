package header

import (
	"slices"
	"strings"

	"github.com/aorith/varnishlog-parser/vsl"
)

// Header States
// NOTE: host and location headers are converted to lowercase
// also in HTTP2 all client headers are lowercased
const (
	OriginalHdr = iota // Before a VCL_call headers are original (as sent by the client or before VCL processing)
	AddedHdr           // Non original headers added in VCL (even if modified later in VCL)
	ModifiedHdr        // Original headers modified
	DeletedHdr         // Original headers deleted
)

// Header is a simple header struct
type Header struct {
	header      string
	headerValue string
}

func (h Header) Header() string {
	return h.header
}

func (h Header) HeaderValue() string {
	return h.headerValue
}

// HeaderStates is an alias for []HeaderState with useful methods
type HeaderStates []HeaderState

// OriginalHeaders returns the headers that were originally sent, before VCL processing
func (h HeaderStates) OriginalHeaders() []Header {
	var headers []Header
	for _, hs := range h {
		if !hs.IsOriginalHeader() {
			continue
		}

		headers = append(
			headers,
			Header{header: hs.Header(), headerValue: hs.OriginalValue()},
		)
	}
	return headers
}

// FinalHeaders returns the headers after the VCL modifications
// those headers could be used to send the request to he backed or for the next varnish subroutine
func (h HeaderStates) FinalHeaders() []Header {
	var headers []Header
	for _, hs := range h {
		if hs.State() == DeletedHdr {
			continue
		}

		headers = append(
			headers,
			Header{header: hs.Header(), headerValue: hs.FinalValue()},
		)
	}
	return headers
}

// FindHeader searches for a specific header within HeaderStates.
//
// Parameters:
// - header: the header name to search for
// - original: if true, searches within original headers; otherwise, searches within final headers
// - ignoreCase: if true, performs a case-insensitive comparison
//
// Returns:
// - A pointer to the matching Header if found; otherwise, nil.
func (h HeaderStates) FindHeader(header string, original, ignoreCase bool) *Header {
	compare := func(a, b string) bool { return a == b }
	if ignoreCase {
		compare = strings.EqualFold
	}

	var hdrs []Header
	if original {
		hdrs = h.OriginalHeaders()
	} else {
		hdrs = h.FinalHeaders()
	}

	for _, hdr := range hdrs {
		if compare(header, hdr.Header()) {
			foundHdr := hdr // Store a copy to avoid unexpected results when returning the pointer
			return &foundHdr
		}
	}
	return nil
}

// HeaderState stores the state of a header
type HeaderState struct {
	header        string
	originalValue string
	finalValue    string
	state         int
}

func (hs HeaderState) IsOriginalHeader() bool {
	return hs.state != AddedHdr
}

func (hs HeaderState) Header() string {
	return hs.header
}

func (hs HeaderState) OriginalValue() string {
	return hs.originalValue
}

func (hs HeaderState) FinalValue() string {
	return hs.finalValue
}

func (hs HeaderState) State() int {
	return hs.state
}

func NewHeaderState(records []vsl.Record, responseHdrs bool) HeaderStates {
	var (
		stateHistory []HeaderState
		currentState HeaderState
	)
	seenHeaders := make(map[string]HeaderState)
	foundCall := false

	for _, record := range records {
		switch headerRecord := record.(type) {
		case vsl.VCLCallRecord:
			if responseHdrs {
				// After the following calls Resp/Beresp headers can be modified in VCL
				if headerRecord.Value() == "DELIVER" || headerRecord.Value() == "BACKEND_RESPONSE" {
					foundCall = true
				}
			} else {
				foundCall = true
			}

		case vsl.ReqHeaderRecord, vsl.BereqHeaderRecord, vsl.RespHeaderRecord, vsl.BerespHeaderRecord:
			if responseHdrs {
				switch headerRecord.(type) {
				case vsl.ReqHeaderRecord, vsl.BereqHeaderRecord:
					continue
				}
			} else {
				switch headerRecord.(type) {
				case vsl.RespHeaderRecord, vsl.BerespHeaderRecord:
					continue
				}
			}

			hdr := headerRecord.(vsl.HeaderRecord)
			originalState, wasSeen := seenHeaders[hdr.Header()]
			originalValue := hdr.HeaderValue()
			if wasSeen {
				originalValue = originalState.OriginalValue()
			}

			if foundCall {
				if wasSeen && originalState.IsOriginalHeader() {
					// Modified header
					currentState = newState(hdr.Header(), originalValue, hdr.HeaderValue(), ModifiedHdr)
				} else {
					// Added header
					currentState = newState(hdr.Header(), originalValue, hdr.HeaderValue(), AddedHdr)
				}
			} else {
				// Original client header before VCL processing
				currentState = newState(hdr.Header(), originalValue, hdr.HeaderValue(), OriginalHdr)
			}

			stateHistory = append(stateHistory, currentState)
			seenHeaders[currentState.Header()] = currentState

		case vsl.ReqUnsetRecord, vsl.BereqUnsetRecord, vsl.RespUnsetRecord, vsl.BerespUnsetRecord:
			if responseHdrs {
				switch headerRecord.(type) {
				case vsl.ReqUnsetRecord, vsl.BereqUnsetRecord:
					continue
				}
			} else {
				switch headerRecord.(type) {
				case vsl.RespUnsetRecord, vsl.BerespUnsetRecord:
					continue
				}
			}

			hdr := headerRecord.(vsl.HeaderRecord)
			originalState, wasSeen := seenHeaders[hdr.Header()]

			if wasSeen {
				if originalState.IsOriginalHeader() {
					// Deleted header from original request
					currentState = newState(originalState.Header(), originalState.OriginalValue(), hdr.HeaderValue(), DeletedHdr)
					stateHistory = append(stateHistory, currentState)
					seenHeaders[currentState.Header()] = currentState
				} else {
					// Discard non-original headers that were added and then removed
					delete(seenHeaders, hdr.Header())
				}
			}
		}
	}

	return consolidateHeaderStates(stateHistory, seenHeaders)
}

// Helper function to create a new HeaderState
func newState(header, originalValue, finalValue string, state int) HeaderState {
	return HeaderState{
		header:        header,
		originalValue: originalValue,
		finalValue:    finalValue,
		state:         state,
	}
}

// Consolidate header states by retaining only the final state for each header
func consolidateHeaderStates(history []HeaderState, seen map[string]HeaderState) HeaderStates {
	uniqueResults := make(map[string]struct{})
	var finalResults []HeaderState

	// Traverse state changes in reverse to keep the last state encountered
	for i := len(history) - 1; i >= 0; i-- {
		hdrState := history[i]
		_, existsInSeen := seen[hdrState.Header()]
		_, alreadyIncluded := uniqueResults[hdrState.Header()]
		if existsInSeen && !alreadyIncluded {
			finalResults = append(finalResults, hdrState)
			uniqueResults[hdrState.Header()] = struct{}{}
		}
	}

	// Reverse finalResults to restore chronological order
	slices.Reverse(finalResults)
	return finalResults
}
