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

func TestHitForMissLabel(t *testing.T) {
	const rawLog = `*   << Request  >> 30
-   Begin          req 1 rxreq
-   VCL_call       RECV
-   VCL_return     hash
-   VCL_call       HASH
-   VCL_return     lookup
-   HitMiss        41609675 29.975025
-   VCL_call       MISS
-   VCL_return     fetch
-   VCL_call       DELIVER
-   VCL_return     deliver
-   RespStatus     200
-   End
`

	p := vsl.NewTransactionParser(strings.NewReader(rawLog))

	ts, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse() failed: %s", err)
	}

	tx := ts.UniqueRootParents(false)[0]
	d := render.Sequence(ts, tx, render.SequenceConfig{})

	if !strings.Contains(d, "HitMiss") {
		t.Errorf("Sequence(): expected a HitMiss label, got: %s", d)
	}

	if !strings.Contains(d, "ObjVXID") {
		t.Errorf("Sequence(): expected HitMiss record details (ObjVXID/TTL), got: %s", d)
	}
}

func TestPlainMissLabel(t *testing.T) {
	const rawLog = `*   << Request  >> 31
-   Begin          req 1 rxreq
-   VCL_call       RECV
-   VCL_return     hash
-   VCL_call       HASH
-   VCL_return     lookup
-   VCL_call       MISS
-   VCL_return     fetch
-   VCL_call       DELIVER
-   VCL_return     deliver
-   RespStatus     200
-   End
`

	p := vsl.NewTransactionParser(strings.NewReader(rawLog))

	ts, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse() failed: %s", err)
	}

	tx := ts.UniqueRootParents(false)[0]
	d := render.Sequence(ts, tx, render.SequenceConfig{})

	if strings.Contains(d, "HitMiss") {
		t.Errorf("Sequence(): did not expect a HitMiss label without a HitMiss record, got: %s", d)
	}

	if !strings.Contains(d, "MISS") {
		t.Errorf("Sequence(): expected a plain MISS label, got: %s", d)
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
