// SPDX-License-Identifier: MIT

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
	ts, err := p.Parse()
	if err != nil {
		t.Errorf("Parse() failed %s", err)
	}

	tx := ts.UniqueRootParents()[0]
	d := render.SequenceDiagram(ts, tx)
	txt := "LINKED CHILD TX NOT FOUND"
	if !strings.Contains(d, txt) {
		t.Errorf("SequenceDiagram() of VCLMissingChild1: expected text %q", txt)
	}
}

func TestLinkLoop(t *testing.T) {
	p := vsl.NewTransactionParser(strings.NewReader(assets.VCLLinkLoop))
	ts, err := p.Parse()
	if err != nil {
		t.Errorf("Parse() failed %s", err)
	}

	ts.GroupRelatedTransactions()

	rootParents := ts.UniqueRootParents()
	if len(rootParents) != 1 {
		t.Errorf("txsSet.UniqueRootParents(): wanted: 1, got: %d", len(rootParents))
	}

	// tx := rootParents[0]
	tx := ts.Transactions()[0]
	d := render.SequenceDiagram(ts, tx)
	txt := "LINKED CHILD TX NOT FOUND"
	if !strings.Contains(d, txt) {
		t.Errorf("SequenceDiagram() of VCLMissingChild1: expected text %q", txt)
	}
}
