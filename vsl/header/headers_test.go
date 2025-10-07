package header

import (
	"fmt"
	"strings"
	"testing"

	"github.com/aorith/varnishlog-parser/vsl"
)

const testVCL = `**  << Request  >> 2
--  Begin          req 1 rxreq
--  ReqMethod      GET
--  ReqURL         /esi/
--  ReqProtocol    HTTP/1.1
--  ReqHeader      Host: www.example1.com
--  ReqHeader      Accept: */*
--  ReqHeader      Test-a: abc
--  ReqHeader      Test-b: x
--  VCL_call       RECV
--  ReqURL         /esi/?test=1
--  ReqUnset       Host: www.example1.com
--  ReqHeader      host: www.example2.com
--  ReqHeader      test-gone: going-to-be-deleted
--  ReqUnset       Test-b: x
--  ReqUnset       Test-a: abc
--  ReqHeader      Test-a: cba
--  ReqHeader      test-x: new
--  ReqHeader      test-y: new
--  ReqUnset       test-x: deleted
--  ReqUnset       test-gone: going-to-be-deleted
--  ReqHeader      test-x: final
--  End
`

func TestHeaderState(t *testing.T) {
	p := vsl.NewTransactionParser(strings.NewReader(testVCL))
	txsSet, err := p.Parse()
	if err != nil {
		t.Errorf("Parse() failed %s", err)
	}

	tx := txsSet.Transactions()[0]

	headerState := NewHeaderState(tx.LogRecords(), false)
	wanted := HeaderStates{
		{
			name:        "Accept",
			originalValue: "*/*",
			finalValue:    "*/*",
			state:         OriginalHdr,
		},
		{
			name:        "Host",
			originalValue: "www.example1.com",
			finalValue:    "www.example2.com",
			state:         ModifiedHdr,
		},
		{
			name:        "Test-B",
			originalValue: "x",
			finalValue:    "x",
			state:         DeletedHdr,
		},
		{
			name:        "Test-A",
			originalValue: "abc",
			finalValue:    "cba",
			state:         ModifiedHdr,
		},
		{
			name:        "Test-Y",
			originalValue: "new",
			finalValue:    "new",
			state:         AddedHdr,
		},
		{
			name:        "Test-X",
			originalValue: "final", // non-client headers do not keep the original value
			finalValue:    "final",
			state:         AddedHdr,
		},
	}

	isEqual := func(a, b HeaderStates) error {
		if len(a) != len(b) {
			return fmt.Errorf("HeaderState(): slices len, wanted: %d, got: %d", len(a), len(b))
		}

		for i := range a {
			if a[i] != b[i] {
				return fmt.Errorf("HeaderState(): wanted: %v, got: %v", a[i], b[i])
			}
		}
		return nil
	}

	err = isEqual(wanted, headerState)
	if err != nil {
		t.Errorf("%s", err)
	}
}

func TestClientAndFinalHeaders(t *testing.T) {
	p := vsl.NewTransactionParser(strings.NewReader(testVCL))
	txsSet, err := p.Parse()
	if err != nil {
		t.Errorf("Parse() failed %s", err)
	}

	tx := txsSet.Transactions()[0]

	headerState := NewHeaderState(tx.LogRecords(), false)

	isEqual := func(a, b []Header) error {
		if len(a) != len(b) {
			return fmt.Errorf("Client/FinalHeaders(): slices len, wanted: %d, got: %d", len(a), len(b))
		}

		for i := range a {
			if a[i] != b[i] {
				return fmt.Errorf("Client/FinalHeaders(): wanted: %v, got: %v", a[i], b[i])
			}
		}
		return nil
	}

	clientHeaders := headerState.OriginalHeaders()
	wantedClientHeaders := []Header{
		{
			name:      "Accept",
			value: "*/*",
		},
		{
			name:      "Host",
			value: "www.example1.com",
		},
		{
			name:      "Test-B",
			value: "x",
		},
		{
			name:      "Test-A",
			value: "abc",
		},
	}

	err = isEqual(wantedClientHeaders, clientHeaders)
	if err != nil {
		t.Errorf("%v", err)
	}

	finalHeaders := headerState.FinalHeaders()
	wantedFinalHeaders := []Header{
		{
			name:      "Accept",
			value: "*/*",
		},
		{
			name:      "Host",
			value: "www.example2.com",
		},
		{
			name:      "Test-A",
			value: "cba",
		},
		{
			name:      "Test-Y",
			value: "new",
		},
		{
			name:      "Test-X",
			value: "final",
		},
	}

	err = isEqual(wantedFinalHeaders, finalHeaders)
	if err != nil {
		t.Errorf("%v", err)
	}
}
