// SPDX-License-Identifier: MIT

// Package vsl, reference: https://varnish-cache.org/docs/trunk/reference/vsl.html
package vsl

import (
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/aorith/varnishlog-parser/vsl/tags"
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
	TXID        TXID     // Custom transaction id: {vxid}-{type}-{reason}[-{ESILevel}] - eg: 33030-req-esi-1
	VXID        VXID     // Transaction ID
	Level       int      // Transaction level
	ESILevel    int      // ESI level, 0 if not an ESI request
	TXType      TxType   // Session, Request, BeReq
	RawLog      string   // Raw log string
	Records     []Record // VSL log records
	ReqHeaders  Headers  // Request Headers
	RespHeaders Headers  // Response Headers
	Parent      VXID     // Parent ID
	Children    []VXID   // Transaction VXIDs which are children of this transaction
}

// RecordByTag returns the the first or last record with the given tag.
// If first is true, it returns the first occurrence; otherwise, it returns the last.
// It returns nil if no record matches the tag.
func (t *Transaction) RecordByTag(tag string, first bool) Record {
	var record Record
	for _, r := range t.Records {
		if r.GetTag() != tag {
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
	for _, r := range t.Records {
		if r.GetTag() != tag {
			continue
		}
		value = r.GetRawValue()
		if first {
			break
		}
	}
	return value
}

// GetBackendConnStr is a helper function to obtain the backend in the format <HOST>:<PORT>
// returns an empty string if not found
func (t *Transaction) GetBackendConnStr() string {
	if t.TXType != TxTypeBereq {
		return ""
	}
	r := t.RecordByTag(tags.BackendOpen, true)
	if r == nil {
		return ""
	}
	record := r.(BackendOpenRecord)
	return fmt.Sprintf("%s:%d", record.RemoteAddr.String(), record.RemotePort)
}

// StartTime is a helper function that returns the approximate start time of the transaction
// since varnishlog does not record the exact start time of a transaction the first timestamp
// or sessopen record is used instead
func (t *Transaction) StartTime() time.Time {
	var startTime time.Time
	if t.TXType == TxTypeSession {
		r := t.RecordByTag(tags.SessOpen, true)
		record, ok := r.(SessOpenRecord)
		if ok {
			startTime = record.SessionStart
		}
	} else {
		r := t.RecordByTag(tags.Timestamp, true)
		record, ok := r.(TimestampRecord)
		if ok {
			startTime = record.StartTime
		}
	}

	return startTime
}

// EndTime is a helper function that returns the approximate end time of the transaction
// since varnishlog does not record the exact end time of a transaction the last timestamp
// or sessclose record is used instead
func (t *Transaction) EndTime() time.Time {
	var endTime time.Time
	if t.TXType == TxTypeSession {
		r := t.RecordByTag(tags.SessClose, false)
		record, ok := r.(SessCloseRecord)
		if ok {
			endTime = t.StartTime().Add(record.Duration)
		}
	} else {
		r := t.RecordByTag(tags.Timestamp, false)
		record, ok := r.(TimestampRecord)
		if ok {
			endTime = record.AbsoluteTime
		}
	}

	return endTime
}

// Duration returns the approximate duration of the transaction
func (t *Transaction) Duration() time.Duration {
	return t.EndTime().Sub(t.StartTime())
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
		VXID:        vxid,
		Level:       level,
		TXType:      txType,
		RawLog:      line,
		ReqHeaders:  make(map[string]Header),
		RespHeaders: make(map[string]Header),
	}, nil
}

// NewMissingTransaction initializes a dummy transaction that
// is missing from the VSL logs using a Link tag record
func NewMissingTransaction(r LinkRecord) *Transaction {
	var txType TxType
	switch r.TXType {
	case "sess":
		txType = TxTypeSession
	case "bereq":
		txType = TxTypeBereq
	default:
		txType = TxTypeRequest
	}

	return &Transaction{
		TXID:   r.TXID,
		TXType: txType,
		Records: []Record{
			BaseRecord{Tag: "__MISSING", RawValue: "This transaction is not present in the provided VSL logs"},
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
func (t TransactionSet) Transactions() []*Transaction {
	txs := make([]*Transaction, 0, len(t.txs))
	for _, tx := range t.txs {
		txs = append(txs, tx)
	}

	sort.Slice(txs, func(i, j int) bool {
		if txs[i].VXID != txs[j].VXID {
			return txs[i].VXID < txs[j].VXID
		}
		if txs[i].Level != txs[j].Level {
			return txs[i].Level < txs[j].Level
		}
		return txs[i].ESILevel < txs[j].ESILevel
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
	if slices.Contains(p.Children, child) {
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
	for _, c := range tx.Children {
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
		if i != 0 && tx.TXType == TxTypeSession {
			s.WriteString("\n")
		}

		s.WriteString(fmt.Sprintf("%s\n", tx.RawLog))

		for _, r := range tx.Records {
			s.WriteString(fmt.Sprintf("%s\n", r.GetRawLog()))
		}
		s.WriteString("\n")
	}

	return s.String()
}

// RawLogForTx returns the VSL raw log for this transactions and optionally its children
func (t TransactionSet) RawLogForTx(tx *Transaction, includeChildrenTxs bool) string {
	var s strings.Builder
	txs := []*Transaction{tx}
	if includeChildrenTxs {
		txs = append(txs, collectAllChildren(&t, tx)...)
	}

	for i, tx := range txs {
		if i != 0 && tx.TXType == TxTypeSession {
			s.WriteString("\n")
		}

		s.WriteString(fmt.Sprintf("%s\n", tx.RawLog))

		for _, r := range tx.Records {
			s.WriteString(fmt.Sprintf("%s\n", r.GetRawLog()))
		}
		s.WriteString("\n")
	}

	return s.String()
}

func (t TransactionSet) GroupRelatedTransactions() [][]*Transaction {
	roots := t.UniqueRootParents(true)

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
// If includeSession is false transactions of type session are excluded
func (t TransactionSet) RootParent(tx *Transaction, includeSession bool) *Transaction {
	var rootParent func(tx *Transaction, maxDepth, depth int) *Transaction
	rootParent = func(tx *Transaction, maxDepth, depth int) *Transaction {
		if !includeSession && tx.TXType == TxTypeSession {
			return nil
		}
		if tx.Parent == 0 {
			return tx
		}
		depth += 1
		if depth > maxDepth {
			slog.Warn("RootParent() possible loop detected", "transaction", tx.TXID, "depth", depth)
			return tx
		}
		parent := t.txs[tx.Parent]
		if parent == nil {
			slog.Debug("RootParent() parent linked but not present in tx map", "child", tx.TXID, "parent", tx.Parent)
			return tx
		}
		if !includeSession && parent.TXType == TxTypeSession {
			return tx
		}
		return rootParent(parent, maxDepth, depth)
	}

	return rootParent(tx, 100, 0)
}

// UniqueRootParents iterates over an array of transactions and returns an array with only the parent transactions.
// If includeSession is false transactions of type session are excluded
func (t TransactionSet) UniqueRootParents(includeSession bool) []*Transaction {
	uniqueParents := make(map[TXID]*Transaction)

	for _, tx := range t.Transactions() {
		if tx == nil {
			continue
		}

		rootParent := t.RootParent(tx, includeSession)
		if rootParent != nil {
			uniqueParents[rootParent.TXID] = rootParent
		}
	}

	parentTxs := make([]*Transaction, 0, len(uniqueParents))
	for _, parent := range uniqueParents {
		parentTxs = append(parentTxs, parent)
	}

	sort.Slice(parentTxs, func(i, j int) bool {
		return parentTxs[i].VXID < parentTxs[j].VXID
	})

	return parentTxs
}
