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
			return txsSet, fmt.Errorf("expected %s tag, found EOF after %q", tag.Begin, tx.RawLog())
		}
		line = strings.TrimSpace(p.scanner.Text())
		if line == "" {
			return txsSet, fmt.Errorf("expected %s tag, found empty line after %q", tag.Begin, tx.RawLog())
		}

		r, err := processRecord(line)
		if err != nil {
			return txsSet, err
		}
		if r.Tag() != tag.Begin {
			return txsSet, fmt.Errorf("expected %s tag, found %q on line %q", tag.Begin, r.Tag(), line)
		}
		// Finish missing Tx field data obtained from the Begin tag
		br := r.(BeginRecord)
		tx.esiLevel = br.ESILevel()
		tx.txid = parseTXID(tx.VXID(), br.Type(), br.ESILevel())
		tx.logRecords = append(tx.logRecords, br)

		// Parse the remaining tags
		complete := false
		vclCallExecuted := false
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

			switch record := r.(type) {
			case VCLCallRecord:
				vclCallExecuted = true

			case StatusRecord:
				// When a status record is received, the state is on the initial Resp or Beresp before any VCL manipulation
				vclCallExecuted = false

			case LinkRecord:
				// Add children to the transaction so they are updated later
				// with the actual transaction (if found)
				lr := r.(LinkRecord)
				childTXID := parseTXID(lr.VXID(), lr.Type(), lr.ESILevel())
				tx.children[childTXID] = &Transaction{level: -1}

			case BeginRecord:
				// A Begin tag was found in the middle of a transaction
				return txsSet, fmt.Errorf("incorrect log: Another %q tag was found in the middle of the transaction %q", tag.Begin, tx.RawLog())

			case HeaderRecord:
				var headers Headers
				if record.IsRespHeader() {
					headers = tx.RespHeaders()
				} else {
					headers = tx.ReqHeaders()
				}

				if vclCallExecuted {
					if headers.Get(record.Name(), false) == "" {
						// Header does not exist, mark it as added
						headers.Add(record.Name(), record.Value(), HdrStateAdded)
					} else {
						// Header exist, add it as modified, VCL 'set' and 'unset' remove
						// all the previous values
						headers.Add(record.Name(), record.Value(), HdrStateModified)
					}
				} else {
					// Received headers
					headers.Add(record.Name(), record.Value(), HdrStateReceived)
				}

			case HeaderUnsetRecord:
				if record.IsRespHeader() {
					tx.RespHeaders().Delete(record.Name())
				} else {
					tx.ReqHeaders().Delete(record.Name())
				}

			}

			// Check if the tx is complete, this is outside of the switch case to be able to break the for loop
			if r.Tag() == tag.End {
				txsSet.txs = append(txsSet.txs, tx)
				txsSet.txsMap[tx.TXID()] = tx
				complete = true
				break
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

		// Headers
	case tag.ReqHeader, tag.RespHeader, tag.BereqHeader, tag.BerespHeader, tag.ObjHeader:
		return NewHeaderRecord(blr)
	case tag.ObjUnset, tag.ReqUnset, tag.RespUnset, tag.BereqUnset, tag.BerespUnset:
		return NewHeaderUnsetRecord(blr)

	case tag.ReqMethod, tag.BereqMethod:
		return MethodRecord{BaseRecord: blr}, nil
	case tag.ReqProtocol, tag.RespProtocol, tag.BereqProtocol, tag.BerespProtocol, tag.ObjProtocol:
		return ProtocolRecord{BaseRecord: blr}, nil
	case tag.BackendOpen:
		return NewBackendOpenRecord(blr)
	case tag.BackendStart:
		return NewBackendStartRecord(blr)
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
	case tag.HitMiss:
		return NewHitMissRecord(blr)
	case tag.TTL:
		return NewTTLRecord(blr)
	case tag.VCLLog:
		return NewVCLLogRecord(blr)
	case tag.Storage:
		return NewStorageRecord(blr)
	case tag.FetchBody:
		return NewFetchBodyRecord(blr)
	case tag.SessOpen:
		return NewSessOpenRecord(blr)
	case tag.SessClose:
		return NewSessCloseRecord(blr)
	case tag.Gzip:
		return NewGzipRecord(blr)
	case tag.VCLCall:
		return VCLCallRecord{BaseRecord: blr}, nil
	case tag.VCLReturn:
		return VCLReturnRecord{BaseRecord: blr}, nil
	case tag.VCLUse:
		return VCLUseRecord{BaseRecord: blr}, nil
	case tag.Error:
		return ErrorRecord{BaseRecord: blr}, nil
	default:
		log.Printf("Unknown tag %q", t)
		return blr, nil
	}
}
