// SPDX-License-Identifier: MIT

package vsl_test

import (
	"strings"
	"testing"

	"github.com/aorith/varnishlog-parser/assets"
	"github.com/aorith/varnishlog-parser/vsl"
)

const (
	testVCL2 = `*12* << Request  >> 40000
-12- Begin          req 39999 esi 10
-12- End
`

	// No End tag
	testVCL3 = `** << Request  >> 39
-- Begin          req 38 esi 1
`

	// VCL_return in place of Begin
	testVCL4 = `** << Request  >> 41
-- VCL_return     hash
-- End
`

	// No Begin or End tags
	testVCL5 = `** << Request  >> 41
`
)

func areTXIDSlicesEqual(a, b []vsl.TXID) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

func TestTransactions(t *testing.T) {
	p := vsl.NewTransactionParser(strings.NewReader(assets.VCLComplete1))
	ts, err := p.Parse()
	if err != nil {
		t.Errorf("Parse() failed %s", err)
	}
	txs := ts.Transactions()

	tx := ts.GetTX(vsl.VXID(33030))
	children := []vsl.TXID{}
	for _, c := range ts.SortedChildren(tx) {
		children = append(children, c.TXID())
	}

	wantedChildren := 3
	if len(children) != wantedChildren {
		t.Errorf("Children len - wanted: %d, got: %d", wantedChildren, len(children))
	}

	txFromMap := ts.GetTX(tx.VXID())
	childrenFromMap := []vsl.TXID{}
	for _, c := range ts.SortedChildren(txFromMap) {
		childrenFromMap = append(childrenFromMap, c.TXID())
	}

	if !areTXIDSlicesEqual(children, childrenFromMap) {
		t.Errorf("TransactionsMap: Transactions children are not equal between the slice and the map: %v != %v", children, childrenFromMap)
	}

	// RootParent() check for VCLComplete1
	rootTx := txs[0]  // << Session  >> 1
	childTx := txs[4] // *4* << BeReq    >> 5
	if ts.RootParent(childTx).TXID() != rootTx.TXID() {
		t.Errorf("RootParent(): wanted: %v, got: %v", rootTx.TXID(), ts.RootParent(childTx).TXID())
	}

	// GroupRelatedTransactions() check for VCLComplete1 which has 24 transactions
	// with 4 groups of related transactions
	wantedTotal := 25
	wantedGroups := 5
	txsGroup := ts.GroupRelatedTransactions()
	if len(txsGroup) != wantedGroups {
		t.Errorf("GroupRelatedTransactions(): (group count) wanted: %d, got: %d", wantedGroups, len(txsGroup))
	}

	count := 0
	for _, g := range txsGroup {
		count = count + len(g)
	}
	if count != wantedTotal {
		t.Errorf("GroupRelatedTransactions(): (txs count) wanted: %d, got: %d", wantedTotal, count)
	}
}

func TestTransactions2(t *testing.T) {
	p := vsl.NewTransactionParser(strings.NewReader(testVCL2))
	txsSet, err := p.Parse()
	if err != nil {
		t.Errorf("Parse() failed %s", err)
	}

	wantedCount := 1
	if len(txsSet.Transactions()) != wantedCount {
		t.Errorf("Incorrect len, expected %d, got %d", wantedCount, len(txsSet.Transactions()))
	}

	tx := txsSet.GetTX(vsl.VXID(40000))
	if tx == nil {
		t.Errorf("Transaction not found, got nil")
		return
	}

	wantedLevel := 12
	if tx.Level() != wantedLevel {
		t.Errorf("Level() wanted: %d, got: %d", wantedLevel, tx.Level())
	}
}

func TestIncompleteTransaction(t *testing.T) {
	p := vsl.NewTransactionParser(strings.NewReader(testVCL3))
	_, err := p.Parse()
	if err == nil {
		t.Errorf("Parse() VCL3 should fail, but succeeded")
	}

	p = vsl.NewTransactionParser(strings.NewReader(testVCL4))
	_, err = p.Parse()
	if err == nil {
		t.Errorf("Parse() VCL4 should fail, but succeeded")
	}

	p = vsl.NewTransactionParser(strings.NewReader(testVCL5))
	_, err = p.Parse()
	if err == nil {
		t.Errorf("Parse() VCL4 should fail, but succeeded")
	}
}
