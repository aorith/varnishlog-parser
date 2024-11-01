package render_test

import (
	"strings"
	"testing"

	"github.com/aorith/varnishlog-parser/assets"
	"github.com/aorith/varnishlog-parser/pkg/render"
	"github.com/aorith/varnishlog-parser/vsl"
)

func TestMissingChild(t *testing.T) {
	p := vsl.NewTransactionParser(strings.NewReader(assets.VCLMissingChild1))
	txsSet, err := p.Parse()
	if err != nil {
		t.Errorf("Parse() failed %s", err)
	}

	tx := txsSet.UniqueRootParents()[0]
	d := render.SequenceDiagram(tx)
	txt := "LINKED CHILD TX NOT FOUND"
	if !strings.Contains(d, txt) {
		t.Errorf("SequenceDiagram() of VCLMissingChild1: expected text %q", txt)
	}
}

func TestLinkLoop(t *testing.T) {
	p := vsl.NewTransactionParser(strings.NewReader(assets.VCLLinkLoop))
	txsSet, err := p.Parse()
	if err != nil {
		t.Errorf("Parse() failed %s", err)
	}

	txsSet.GroupRelatedTransactions()

	rootParents := txsSet.UniqueRootParents()
	if len(rootParents) != 1 {
		t.Errorf("txsSet.UniqueRootParents(): wanted: 1, got: %d", len(rootParents))
	}

	// tx := rootParents[0]
	tx := txsSet.Transactions()[0]
	d := render.SequenceDiagram(tx)
	txt := "LINKED CHILD TX NOT FOUND"
	if !strings.Contains(d, txt) {
		t.Errorf("SequenceDiagram() of VCLMissingChild1: expected text %q", txt)
	}
}
