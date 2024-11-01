package vsl

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/aorith/varnishlog-parser/vsl/tag"
)

type transactionParser struct {
	scanner *bufio.Scanner
}

func NewTransactionParser(r io.Reader) *transactionParser {
	return &transactionParser{
		scanner: bufio.NewScanner(r),
	}
}

func (p *transactionParser) Parse() (TransactionSet, error) {
	txsSet := TransactionSet{
		txsMap: make(map[string]*Transaction),
	}

	for p.scanner.Scan() {
		line := strings.TrimSpace(p.scanner.Text())
		parts := strings.Fields(line)

		// Look for the start of a transaction, eg:
		// *   << Session  >> 16812342
		// **  << Request  >> 4
		if len(parts) != 5 || parts[0][0] != '*' || parts[1][0] != '<' {
			continue
		}

		tx, err := NewTransaction(line)
		if err != nil {
			return txsSet, err
		}

		// Expect a Begin tag after the start of the transaction, eg:
		// --- Begin          req 2 esi 1
		if !p.scanner.Scan() {
			return txsSet, fmt.Errorf("Expected begin tag, found EOF")
		}
		line = strings.TrimSpace(p.scanner.Text())

		r, err := processRecord(line)
		if err != nil {
			return txsSet, err
		}
		if r.Tag() != tag.Begin {
			return txsSet, fmt.Errorf("Expected begin tag, found %q on line %q", r.Tag(), line)
		}
		// Finish missing Tx field data obtained from the Begin tag
		br := r.(BeginRecord)
		tx.esiLevel = br.ESILevel()
		tx.txid = parseTXID(tx.VXID(), br.Type(), br.ESILevel())
		tx.logRecords = append(tx.logRecords, br)

		// Parse the remaining tags
		complete := false
		for p.scanner.Scan() {
			line := strings.TrimSpace(p.scanner.Text())
			// Skip empty lines or invalid lines
			if len(strings.Fields(line)) < 2 {
				continue
			}

			r, err := processRecord(line)
			if err != nil {
				return txsSet, err
			}
			tx.logRecords = append(tx.logRecords, r)

			if r.Tag() == tag.End {
				// The tx is complete
				txsSet.txs = append(txsSet.txs, tx)
				txsSet.txsMap[tx.TXID()] = tx
				complete = true
				break
			} else if r.Tag() == tag.Link {
				// Add children to the transaction so they are updated later
				// with the actual transaction (if found)
				lr := r.(LinkRecord)
				childTXID := parseTXID(lr.VXID(), lr.Type(), lr.ESILevel())
				tx.children[childTXID] = &Transaction{level: -1}
			} else if r.Tag() == tag.Begin {
				// A Begin tag was found in the middle of a transaction
				return txsSet, fmt.Errorf("Incorrect log: Another %q tag was found in the middle of the transaction %q", tag.Begin, tx.RawLog())
			}
		}

		if err := p.scanner.Err(); err != nil {
			return txsSet, err
		}

		if !complete {
			return txsSet, fmt.Errorf("Transaction %q finished without %s tag at EOL", tx.RawLog(), tag.End)
		}
	}

	if err := p.scanner.Err(); err != nil {
		return txsSet, err
	}

	// Update parent and children relationships
	for _, currTx := range txsSet.Transactions() {
		for childTXID := range currTx.Children() {
			child, childExists := txsSet.txsMap[childTXID]
			if childExists {
				child.parent = currTx
				currTx.children[childTXID] = child
			}
		}
	}

	// Delete children not found in the complete varnishlog log
	for _, currTx := range txsSet.Transactions() {
		for childTXID, child := range currTx.Children() {
			if child.Level() == -1 {
				delete(currTx.Children(), childTXID)
			}
		}
	}

	return txsSet, nil
}

func processRecord(line string) (Record, error) {
	blr, err := NewBaseRecord(line)
	if err != nil {
		return blr, err
	}

	t := blr.Tag()
	switch t {
	case tag.End:
		return EndRecord{BaseRecord: blr}, nil
	case tag.RespReason, tag.BerespReason:
		return ReasonRecord{BaseRecord: blr}, nil
	case tag.FetchError:
		return FetchErrorRecord{BaseRecord: blr}, nil
	case tag.Begin:
		return NewBeginRecord(blr)
	case tag.Link:
		return NewLinkRecord(blr)
	case tag.ReqHeader:
		return NewReqHeaderRecord(blr)
	case tag.RespHeader:
		return NewRespHeaderRecord(blr)
	case tag.BereqHeader:
		return NewBereqHeaderRecord(blr)
	case tag.BerespHeader:
		return NewBerespHeaderRecord(blr)
	case tag.ObjHeader:
		return NewObjHeaderRecord(blr)
	case tag.ObjUnset:
		return NewObjUnsetRecord(blr)
	case tag.ReqUnset:
		return NewReqUnsetRecord(blr)
	case tag.RespUnset:
		return NewRespUnsetRecord(blr)
	case tag.BereqUnset:
		return NewBereqUnsetRecord(blr)
	case tag.BerespUnset:
		return NewBerespUnsetRecord(blr)
	case tag.ReqMethod, tag.BereqMethod:
		return MethodRecord{BaseRecord: blr}, nil
	case tag.ReqProtocol, tag.RespProtocol, tag.BereqProtocol, tag.BerespProtocol, tag.ObjProtocol:
		return ProtocolRecord{BaseRecord: blr}, nil
	case tag.BackendOpen:
		return NewBackendOpenRecord(blr)
	case tag.BackendClose:
		return NewBackendCloseRecord(blr)
	case tag.ReqAcct, tag.BereqAcct:
		return NewAcctRecord(blr)
	case tag.Timestamp:
		return NewTimestampRecord(blr)
	case tag.ReqStart:
		return NewReqStartRecord(blr)
	case tag.ReqURL, tag.BereqURL:
		return NewURLRecord(blr)
	case tag.Filters:
		return NewFiltersRecord(blr)
	case tag.RespStatus, tag.BerespStatus, tag.ObjStatus:
		return NewStatusRecord(blr)
	case tag.Length:
		return NewLengthRecord(blr)
	case tag.Hit:
		return NewHitRecord(blr)
	case tag.TTL:
		return NewTTLRecord(blr)
	case tag.VCL_Log:
		return NewVCLLogRecord(blr)
	case tag.Storage:
		return NewStorageRecord(blr)
	case tag.Fetch_Body:
		return NewFetchBodyRecord(blr)
	case tag.SessOpen:
		return NewSessOpenRecord(blr)
	case tag.SessClose:
		return NewSessCloseRecord(blr)
	case tag.VCL_call:
		return VCLCallRecord{BaseRecord: blr}, nil
	case tag.VCL_return:
		return VCLReturnRecord{BaseRecord: blr}, nil
	case tag.VCL_use:
		return VCLUseRecord{BaseRecord: blr}, nil
	default:
		log.Printf("Unknown tag %q", t)
		return blr, nil
	}
}
