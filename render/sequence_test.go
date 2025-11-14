// SPDX-License-Identifier: MIT

package render_test

import (
	"strings"
	"testing"

	"github.com/aorith/varnishlog-parser/assets"
	"github.com/aorith/varnishlog-parser/render"
	"github.com/aorith/varnishlog-parser/vsl"
)

func TestMissingChild(t *testing.T) {
	p := vsl.NewTransactionParser(strings.NewReader(assets.VCLMissingChild1))
	ts, err := p.Parse()
	if err != nil {
		t.Errorf("Parse() failed %s", err)
	}

	tx := ts.UniqueRootParents(false)[0]
	d := render.Sequence(ts, tx, render.SequenceConfig{})
	txt := "child tx not found"
	if !strings.Contains(d, txt) {
		t.Errorf("Sequence() of VCLMissingChild1: expected text %q, got %s", txt, d)
	}
}

func TestLinkLoop(t *testing.T) {
	p := vsl.NewTransactionParser(strings.NewReader(assets.VCLLinkLoop))
	ts, err := p.Parse()
	if err != nil {
		t.Errorf("Parse() failed %s", err)
	}

	ts.GroupRelatedTransactions()

	rootParents := ts.UniqueRootParents(false)
	if len(rootParents) != 1 {
		t.Errorf("txsSet.UniqueRootParents(): wanted: 1, got: %d", len(rootParents))
	}

	tx := rootParents[0]
	d := render.Sequence(ts, tx, render.SequenceConfig{})
	txt := "child tx not found"
	if !strings.Contains(d, txt) {
		t.Errorf("Sequence() of VCLMissingChild1: expected text %q, got %s", txt, d)
	}
}
