package vsl

import (
	"fmt"
	"log"
	"reflect"
	"slices"
	"sort"
	"strings"
)

// Reference: https://varnish-cache.org/docs/trunk/reference/vsl.html

const (
	TxTypeSession = "Session"
	TxTypeRequest = "Request"
	TxTypeBereq   = "BeReq"
)

var allTxTypes = []string{TxTypeSession, TxTypeRequest, TxTypeBereq}

func isValidTxType(txType string) bool {
	return slices.Contains(allTxTypes, txType)
}

// TransactionSet groups multiple Varnish transaction logs together
type TransactionSet struct {
	txs    []*Transaction
	txsMap map[string]*Transaction // map[{txid}]*tx
}

func (t TransactionSet) Transactions() []*Transaction {
	return t.txs
}

func (t TransactionSet) TransactionsMap() map[string]*Transaction {
	return t.txsMap
}

// RawLog returns the complete VSL raw log from all the transactions
func (t TransactionSet) RawLog() string {
	var s strings.Builder

	for i, tx := range t.txs {
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
		children := collectAllChildren(r)
		if children != nil {
			txsGroup = append(txsGroup, children...)
		}
		txs = append(txs, txsGroup)
	}

	return txs
}

// UniqueRootParents iterates over an array of transactions and returns an array with only the parent transactions.
func (t TransactionSet) UniqueRootParents() []*Transaction {
	uniqueParents := make(map[string]*Transaction)

	for _, tx := range t.Transactions() {
		if tx == nil {
			continue
		}

		rootParent := tx.RootParent()
		if rootParent != nil {
			uniqueParents[rootParent.TXID()] = rootParent
		}
	}

	parentTxs := make([]*Transaction, 0, len(uniqueParents))
	for _, parent := range uniqueParents {
		parentTxs = append(parentTxs, parent)
	}

	// Sort the resulting slice by TXID
	sort.Slice(parentTxs, func(i, j int) bool {
		return parentTxs[i].TXID() < parentTxs[j].TXID()
	})

	return parentTxs
}

// Transaction represent a singular Varnish transaction log
type Transaction struct {
	txid       string // {vxid}_{type}[_{esiLevel}]
	vxid       VXID
	level      int
	esiLevel   int    // 0 == no ESI
	txType     string // Session, Request, BeReq
	rawLog     string // Raw log string
	logRecords []Record
	parent     *Transaction
	children   map[string]*Transaction // map[{txid}]*tx
}

func (t *Transaction) TXID() string {
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

func (t *Transaction) Type() string {
	return t.txType
}

func (t *Transaction) RawLog() string {
	return t.rawLog
}

func (t *Transaction) LogRecords() []Record {
	return t.logRecords
}

func (t *Transaction) Parent() *Transaction {
	return t.parent
}

func (t *Transaction) Children() map[string]*Transaction {
	return t.children
}

// FullRawLog returns the complete VSL log from this transaction
// if withChildren is true it also includes the log from all its children recursively
func (t *Transaction) FullRawLog(withChildren bool) string {
	var s strings.Builder
	txs := []*Transaction{t}
	if withChildren {
		txs = append(txs, collectAllChildren(t)...)
	}

	for i, tx := range txs {
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

// RootParent returns the root transaction which has no parent
func (t *Transaction) RootParent() *Transaction {
	var rootParent func(tx *Transaction, maxDepth, depth int) *Transaction
	rootParent = func(tx *Transaction, maxDepth, depth int) *Transaction {
		if tx.parent == nil || tx.txid == tx.parent.txid {
			return tx
		}
		depth += 1
		if depth > maxDepth {
			log.Printf("RootParent() possible loop detected at transaction %q - depth: %d\n", tx.TXID(), depth)
			return tx
		}
		return rootParent(tx.parent, maxDepth, depth)
	}

	return rootParent(t, 100, 0)
}

// ChildrenSortedByVXID returns a slice with all the children sorted by VXID
func (t *Transaction) ChildrenSortedByVXID() []*Transaction {
	childrenSlice := make([]*Transaction, 0, len(t.Children()))
	for _, child := range t.Children() {
		childrenSlice = append(childrenSlice, child)
	}

	sort.Slice(childrenSlice, func(i, j int) bool {
		return childrenSlice[i].VXID() < childrenSlice[j].VXID()
	})

	return childrenSlice
}

// FirstRecordOfType returns the first record for the given type
func (t *Transaction) FirstRecordOfType(target any) Record {
	targetType := reflect.TypeOf(target)

	for _, r := range t.LogRecords() {
		if reflect.TypeOf(r) == targetType {
			return r
		}
	}

	return nil
}

// LastRecordOfType returns the last record for the given type
func (t *Transaction) LastRecordOfType(target any) Record {
	var record Record
	targetType := reflect.TypeOf(target)

	for _, r := range t.LogRecords() {
		if reflect.TypeOf(r) == targetType {
			record = r
		}
	}

	return record
}

// FirstRecordOfTag returns the first record for the given tag
func (t *Transaction) FirstRecordOfTag(tag string) Record {
	for _, r := range t.LogRecords() {
		if r.Tag() == tag {
			return r
		}
	}
	return nil
}

// LastRecordOfTag returns the last record for the given tag
func (t *Transaction) LastRecordOfTag(tag string) Record {
	var record Record
	for _, r := range t.LogRecords() {
		if r.Tag() == tag {
			record = r
		}
	}
	return record
}

// NewTransaction initializes a new transaction by parsing the first line of the log
func NewTransaction(line string) (*Transaction, error) {
	parts := strings.Fields(line)
	txType := parts[2]
	if !isValidTxType(txType) {
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
		vxid:     vxid,
		level:    level,
		txType:   txType,
		rawLog:   line,
		children: make(map[string]*Transaction),
	}, nil
}

// NewMissingTransaction initializes a transaction that is missing in the VSL logs
// from a Link tag record
func NewMissingTransaction(r LinkRecord) *Transaction {
	var txType string
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
