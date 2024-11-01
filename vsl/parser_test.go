package vsl_test

import (
	"strings"
	"testing"

	"github.com/aorith/varnishlog-parser/assets"
	"github.com/aorith/varnishlog-parser/vsl"
	"github.com/aorith/varnishlog-parser/vsl/tag"
)

func TestParse(t *testing.T) {
	p := vsl.NewTransactionParser(strings.NewReader(assets.VCLComplete1))
	txsSet, err := p.Parse()
	txs := txsSet.Transactions()
	if err != nil {
		t.Errorf("Parse() failed %s", err)
	}

	if len(txs) != 25 {
		t.Errorf("incorrect len for test case, wanted: %v got: %v", 24, len(txs))
	}

	for i, tx := range txs {
		if tx.LogRecords()[0].Tag() != tag.Begin {
			t.Errorf("wrong tag for first logRecord of txs[%d], wanted: %v got: %v", i, tag.Begin, tx.LogRecords()[0].Tag())
		}
		if tx.LogRecords()[len(tx.LogRecords())-1].Tag() != tag.End {
			t.Errorf("wrong tag for last logRecord of txs[%d], wanted: %v got: %v", i, tag.End, tx.LogRecords()[len(tx.LogRecords())-1].Tag())
		}
	}

	firstTx := txs[0]
	if firstTx.Type() != vsl.TxTypeSession {
		t.Errorf("Type wanted: %v, got: %v", vsl.TxTypeSession, firstTx.Type())
	}

	esiWanted1 := 0
	if firstTx.ESILevel() != esiWanted1 {
		t.Errorf("ESILevel wanted: %v, got: %v", esiWanted1, firstTx.ESILevel())
	}

	levelWanted1 := 1
	if firstTx.Level() != levelWanted1 {
		t.Errorf("Level wanted: %v, got: %v", levelWanted1, firstTx.Level())
	}

	lastTx := txs[len(txs)-1]
	if lastTx.Type() != vsl.TxTypeBereq {
		t.Errorf("Type wanted: %v, got: %v", vsl.TxTypeBereq, lastTx.Type())
	}

	esiWanted2 := 0
	if lastTx.ESILevel() != esiWanted2 {
		t.Errorf("ESILevel wanted: %v, got: %v", esiWanted2, lastTx.ESILevel())
	}

	levelWanted2 := 3
	if lastTx.Level() != levelWanted2 {
		t.Errorf("Level wanted: %v, got: %v", levelWanted2, lastTx.Level())
	}

	esiTx := txs[12]
	esiWanted3 := 2
	if esiTx.ESILevel() != esiWanted3 {
		t.Errorf("ESILevel of tx %v, wanted: %v, got: %v", esiTx.TXID(), esiWanted3, esiTx.ESILevel())
	}
}
