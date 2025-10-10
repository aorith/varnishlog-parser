package vsl_test

import (
	"strings"
	"testing"

	"github.com/aorith/varnishlog-parser/assets"
	"github.com/aorith/varnishlog-parser/pkg/render"
	"github.com/aorith/varnishlog-parser/vsl"
	"github.com/aorith/varnishlog-parser/vsl/tag"
)

func TestParse(t *testing.T) {
	p := vsl.NewTransactionParser(strings.NewReader(assets.VCLComplete1))
	txsSet, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse() failed: %s", err)
	}

	txs := txsSet.Transactions()
	const expectedTxCount = 25
	if len(txs) != expectedTxCount {
		t.Fatalf("incorrect transaction count, wanted: %d, got: %d", expectedTxCount, len(txs))
	}

	// Validate first and last log record tags of each transaction
	for i, tx := range txs {
		first := tx.LogRecords()[0].Tag()
		last := tx.LogRecords()[len(tx.LogRecords())-1].Tag()

		if first != tag.Begin {
			t.Errorf("tx[%d]: first logRecord tag, wanted: %v, got: %v", i, tag.Begin, first)
		}
		if last != tag.End {
			t.Errorf("tx[%d]: last logRecord tag, wanted: %v, got: %v", i, tag.End, last)
		}
	}

	// Validate some specific transactions
	tests := []struct {
		txIndex int
		txType  vsl.TxType
		esi     int
		level   int
	}{
		{0, vsl.TxTypeSession, 0, 1},
		{len(txs) - 1, vsl.TxTypeBereq, 0, 3},
		{12, vsl.TxTypeRequest, 2, 0},
	}

	for _, tt := range tests {
		tx := txs[tt.txIndex]
		if tx.Type() != tt.txType {
			t.Errorf("tx[%d]: type wanted: %v, got: %v", tt.txIndex, tt.txType, tx.Type())
		}
		if tx.ESILevel() != tt.esi {
			t.Errorf("tx[%d]: ESILevel wanted: %v, got: %v", tt.txIndex, tt.esi, tx.ESILevel())
		}
		if tt.level != 0 && tx.Level() != tt.level {
			t.Errorf("tx[%d]: Level wanted: %v, got: %v", tt.txIndex, tt.level, tx.Level())
		}
	}
}

func TestReceivedHeaders(t *testing.T) {
	p := vsl.NewTransactionParser(strings.NewReader(assets.VCLComplete1))
	txsSet, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse() failed: %s", err)
	}

	txs := txsSet.Transactions()
	tx := txs[1]
	if tx.Type() != vsl.TxTypeRequest {
		t.Fatalf("tx[1] type wanted: %v, got: %v", vsl.TxTypeRequest, tx.Type())
	}

	// Convert to HTTPRequest
	hr, err := render.NewHTTPRequest(tx, true, nil)
	if err != nil {
		t.Fatalf("conversion to HTTPRequest failed: %s", err)
	}

	// --  ReqProtocol    HTTP/1.1
	// --  ReqHeader      Host: www.example1.com                           <-- received (ignored)
	// --  ReqHeader      User-Agent: curl/8.7.1                           <-- received
	// --  ReqHeader      Accept: */*                                      <-- received
	// --  ReqHeader      secret:1234                                      <-- received
	// --  ReqHeader      X-Forwarded-For: 1.1.1.1                         <-- received (to-be-merged)
	// --  ReqHeader      X-Forwarded-For: 2.2.2.2                         <-- received (to-be-merged)
	// --  ReqUnset       X-Forwarded-For: 1.1.1.1, 2.2.2.2                <-- received (merged)
	// --  ReqHeader      X-Forwarded-For: 1.1.1.1, 2.2.2.2, 192.168.65.1  <-- processed
	// --  ReqHeader      Via: 1.1 b736436225f7 (Varnish/7.5)              <-- processed
	// --  VCL_call       RECV
	// --  VCL_Log        custom VCL recv
	// --  ReqURL         /esi?turing=imitation-game
	// --  ReqHeader      xid: 262                                         <-- processed
	// --  ReqHeader      X-Test-Header: Test Value                        <-- processed
	// --  ReqUnset       X-Test-Header: Test Value                        <-- processed (deleted)
	// --  VCL_return     hash

	// Received:  4
	// Processed: 6 (received + processed - deleted)

	headers := hr.Headers()
	expected := map[string]string{
		"User-Agent":      "curl/8.7.1",
		"Accept":          "*/*",
		"Secret":          "1234",
		"X-Forwarded-For": "1.1.1.1, 2.2.2.2",
	}

	if len(headers) != len(expected) {
		t.Fatalf("incorrect number of received headers, wanted: %d, got: %d", len(expected), len(headers))
	}

	// Compare expected header values
	for name, want := range expected {
		var got string
		for _, h := range headers {
			if h.Name() == name {
				got = h.Value()
				break
			}
		}
		if got != want {
			t.Errorf("header %q: expected %q, got %q", name, want, got)
		}
	}
}

func TestProcessedHeaders(t *testing.T) {
	p := vsl.NewTransactionParser(strings.NewReader(assets.VCLComplete1))
	txsSet, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse() failed: %s", err)
	}

	txs := txsSet.Transactions()
	tx := txs[1]
	if tx.Type() != vsl.TxTypeRequest {
		t.Fatalf("tx[1] type wanted: %v, got: %v", vsl.TxTypeRequest, tx.Type())
	}

	// Convert to HTTPRequest
	hr, err := render.NewHTTPRequest(tx, false, nil)
	if err != nil {
		t.Fatalf("conversion to HTTPRequest failed: %s", err)
	}

	headers := hr.Headers()
	expected := map[string]string{
		"User-Agent":      "curl/8.7.1",
		"Accept":          "*/*",
		"Secret":          "1234",
		"X-Forwarded-For": "1.1.1.1, 2.2.2.2, 192.168.65.1",
		"Via":             "1.1 b736436225f7 (Varnish/7.5)",
		"Xid":             "262",
	}

	if len(headers) != len(expected) {
		t.Fatalf("incorrect number of processed headers, wanted: %d, got: %d", len(expected), len(headers))
	}

	// Compare expected header values
	for name, want := range expected {
		var got string
		for _, h := range headers {
			if h.Name() == name {
				got = h.Value()
				break
			}
		}
		if got != want {
			t.Errorf("header %q: expected %q, got %q", name, want, got)
		}
	}
}
