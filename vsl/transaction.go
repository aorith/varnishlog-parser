// SPDX-License-Identifier: MIT

// Package vsl, reference: https://varnish-cache.org/docs/trunk/reference/vsl.html
package vsl

import (
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"strings"
)

// TxType represents the type of a Varnish transaction.
type TxType string

const (
	TxTypeSession TxType = "Session"
	TxTypeRequest TxType = "Request"
	TxTypeBereq   TxType = "BeReq"
)

var allTxTypes = []TxType{TxTypeSession, TxTypeRequest, TxTypeBereq}

// Transaction represent a singular Varnish transaction log
type Transaction struct {
	txid        TXID // {vxid}_{type}[_esi_{esiLevel}] - eg: 33030_req_esi_1
	vxid        VXID
	level       int
	esiLevel    int    // 0 == no ESI
	txType      TxType // Session, Request, BeReq
	rawLog      string // Raw log string
	logRecords  []Record
	reqHeaders  Headers // Request Headers
	respHeaders Headers // Response Headers
	parent      VXID
	children    []VXID
}

func (t *Transaction) TXID() TXID {
	return t.txid
}

func (t *Transaction) VXID() VXID {
	return t.vxid
}

func (t *Transaction) Level() int {
	return t.level
}

func (t *Transaction) ESILevel() int {
	return t.esiLevel
}

func (t *Transaction) Type() TxType {
	return t.txType
}

func (t *Transaction) RawLog() string {
	return t.rawLog
}

func (t *Transaction) LogRecords() []Record {
	return t.logRecords
}

func (t *Transaction) ReqHeaders() Headers {
	return t.reqHeaders
}

func (t *Transaction) RespHeaders() Headers {
	return t.respHeaders
}

func (t *Transaction) Parent() VXID {
	return t.parent
}

func (t *Transaction) Children() []VXID {
	return t.children
}

// RecordByTag returns the the first or last record with the given tag.
// If first is true, it returns the first occurrence; otherwise, it returns the last.
// It returns nil if no record matches the tag.
func (t *Transaction) RecordByTag(tag string, first bool) Record {
	var record Record
	for _, r := range t.LogRecords() {
		if r.Tag() != tag {
			continue
		}
		record = r
		if first {
			break
		}
	}
	return record
}

// RecordValueByTag returns the value of the first or last record with the given tag.
// If first is true, it returns the first occurrence; otherwise, it returns the last.
// It returns an empty string if no record matches the tag.
func (t *Transaction) RecordValueByTag(tag string, first bool) string {
	var value string
	for _, r := range t.LogRecords() {
		if r.Tag() != tag {
			continue
		}
		value = r.Value()
		if first {
			break
		}
	}
	return value
}

// NewTransaction initializes a new transaction by parsing the first line of the log
func NewTransaction(line string) (*Transaction, error) {
	parts := strings.Fields(line)
	txType := TxType(parts[2])
	if !slices.Contains(allTxTypes, txType) {
		return nil, fmt.Errorf("unknown transaction of type '%q' - known types: %q", txType, allTxTypes)
	}

	vxid, err := parseVXID(parts[4])
	if err != nil {
		return nil, fmt.Errorf("incorrect vxid found on line '%q', error: %s", parts, err)
	}

	level, err := parseLevel(parts[0])
	if err != nil {
		return nil, err
	}

	return &Transaction{
		vxid:        vxid,
		level:       level,
		txType:      txType,
		rawLog:      line,
		reqHeaders:  make(map[string]Header),
		respHeaders: make(map[string]Header),
	}, nil
}

// NewMissingTransaction initializes a dummy transaction that
// is missing from the VSL logs using a Link tag record
func NewMissingTransaction(r LinkRecord) *Transaction {
	var txType TxType
	switch r.Type() {
	case "sess":
		txType = TxTypeSession
	case "bereq":
		txType = TxTypeBereq
	default:
		txType = TxTypeRequest
	}

	return &Transaction{
		txid:   r.TXID(),
		txType: txType,
		logRecords: []Record{
			BaseRecord{tag: "__MISSING", value: "This transaction is not present in the provided VSL logs"},
		},
	}
}

// TransactionSet groups multiple Varnish transaction logs together
type TransactionSet struct {
	txs map[VXID]*Transaction // map[{vxid}]*tx
}

// TransactionsMap returns the transactions map
func (t TransactionSet) TransactionsMap() map[VXID]*Transaction {
	return t.txs
}

// Transactions returns a sorted slice with all the transactions
func (t *TransactionSet) Transactions() []*Transaction {
	txs := make([]*Transaction, 0, len(t.txs))
	for _, tx := range t.txs {
		txs = append(txs, tx)
	}

	sort.Slice(txs, func(i, j int) bool {
		if txs[i].vxid != txs[j].vxid {
			return txs[i].vxid < txs[j].vxid
		}
		if txs[i].level != txs[j].level {
			return txs[i].level < txs[j].level
		}
		return txs[i].esiLevel < txs[j].esiLevel
	})

	return txs
}

// GetTX returns the transaction by VXID or nil if not found
func (t TransactionSet) GetTX(vxid VXID) *Transaction {
	return t.txs[vxid]
}

// GetChildTX returns the transaction child by VXID or nil if not found
func (t TransactionSet) GetChildTX(parent, child VXID) *Transaction {
	p := t.txs[parent]
	if p == nil {
		return nil
	}
	if slices.Contains(p.children, child) {
		return t.txs[child]
	}
	// Even if the tx is on the set, the given parent does not contain that children
	return nil
}

// SortedChildren returns a sorted slice of all the tx children
func (t TransactionSet) SortedChildren(tx *Transaction) []*Transaction {
	if tx == nil {
		return nil
	}
	txs := make(map[VXID]*Transaction)
	for _, c := range tx.Children() {
		child := t.txs[c]
		if child != nil {
			txs[c] = child
		}
	}
	ts := TransactionSet{txs: txs}
	return ts.Transactions()
}

// RawLog returns the complete VSL raw log from all the transactions
func (t TransactionSet) RawLog() string {
	var s strings.Builder

	for i, tx := range t.Transactions() {
		if i != 0 && tx.Type() == TxTypeSession {
			s.WriteString("\n")
		}

		s.WriteString(fmt.Sprintf("%s\n", tx.RawLog()))

		for _, r := range tx.LogRecords() {
			s.WriteString(fmt.Sprintf("%s\n", r.RawLog()))
		}
		s.WriteString("\n")
	}

	return s.String()
}

func (t TransactionSet) GroupRelatedTransactions() [][]*Transaction {
	roots := t.UniqueRootParents()

	var txs [][]*Transaction
	for _, r := range roots {
		txsGroup := []*Transaction{r}
		children := collectAllChildren(&t, r)
		if children != nil {
			txsGroup = append(txsGroup, children...)
		}
		txs = append(txs, txsGroup)
	}

	return txs
}

// RootParent returns the root transaction which has no parent
func (t *TransactionSet) RootParent(tx *Transaction) *Transaction {
	var rootParent func(tx *Transaction, maxDepth, depth int) *Transaction
	rootParent = func(tx *Transaction, maxDepth, depth int) *Transaction {
		if tx.Parent() == 0 {
			return tx
		}
		depth += 1
		if depth > maxDepth {
			slog.Warn("RootParent() possible loop detected", "transaction", tx.TXID(), "depth", depth)
			return tx
		}
		parent := t.txs[tx.Parent()]
		if parent == nil {
			slog.Debug("RootParent() parent linked but not present in tx map", "child", tx.TXID(), "parent", tx.Parent())
			return tx
		}
		return rootParent(parent, maxDepth, depth)
	}

	return rootParent(tx, 100, 0)
}

// UniqueRootParents iterates over an array of transactions and returns an array with only the parent transactions.
func (t TransactionSet) UniqueRootParents() []*Transaction {
	uniqueParents := make(map[TXID]*Transaction)

	for _, tx := range t.Transactions() {
		if tx == nil {
			continue
		}

		rootParent := t.RootParent(tx)
		if rootParent != nil {
			uniqueParents[rootParent.TXID()] = rootParent
		}
	}

	parentTxs := make([]*Transaction, 0, len(uniqueParents))
	for _, parent := range uniqueParents {
		parentTxs = append(parentTxs, parent)
	}

	sort.Slice(parentTxs, func(i, j int) bool {
		return parentTxs[i].VXID() < parentTxs[j].VXID()
	})

	return parentTxs
}
