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
		complete := false                                 // to check at the end if the transaction finished (found End tag for example)
		clientHeaders := true                             // keep track if we are still parsing client/received headers
		var lastHeaderRecord *HeaderRecord                // required to track client/received headers
		var tempHeaders Headers = make(map[string]Header) // required to track client/received headers
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
				if clientHeaders {
					clientHeaders = false
					// Check what was the last header to select either 'tx.ReqHeaders()' or 'tx.RespHeaders()'
					// prefer this rather that checking if the call is for 'recv', 'miss', 'deliver', etc, as that could be more brittle
					if lastHeaderRecord == nil {
						tempHeaders.Clear() // should be empty already
						continue
					}
					if lastHeaderRecord.IsRespHeader() {
						mergeTempHeaders(tx.RespHeaders(), tempHeaders)
					} else {
						mergeTempHeaders(tx.ReqHeaders(), tempHeaders)
					}
				}

			case StatusRecord:
				// When a status record is received, the state is on the initial Resp or Beresp before any VCL manipulation
				clientHeaders = true

			case LinkRecord:
				// Add children to the transaction so they are updated later
				// with the actual transaction (if found)
				lr := r.(LinkRecord)
				childTXID := parseTXID(lr.VXID(), lr.Type(), lr.ESILevel())
				tx.children[childTXID] = &Transaction{level: -1}

			case BeginRecord:
				// A Begin tag was found in the middle of a transaction
				return txsSet, fmt.Errorf("incorrect log: Another %q tag was found in the middle of the transaction %q", tag.Begin, tx.RawLog())

			// HEADERS: handle parsing of HTTP headers, transactions have two  Headers sets, one for Req and another for Resp requests
			// Varnish also has some built-in VCL that executes after users VCL (if not overridden by a return), but most importantly
			// it has 'core' code (in C) that modifies some headers like X-F-F before any VCL is called. So it is a bit tricky to know
			// if a header comes from the client (eg: curl) or it was Varnish who did that.
			// Ref: https://github.com/varnishcache/varnish-cache/blob/9f02342b455469349e24a88e49550f23c262baaf/bin/varnishd/cache/cache_req_fsm.c#L908-L909

			// For simplicity let's consider that all 'unsets' of headers present at 'isVarnishModifiedHeader()' that
			// happen before a VCL_call are client-sent/received headers.
			case HeaderRecord:
				recordCopy := record
				lastHeaderRecord = &recordCopy

				var headers Headers
				if record.IsRespHeader() {
					headers = tx.RespHeaders()
				} else {
					headers = tx.ReqHeaders()
				}

				if clientHeaders {
					if isVarnishModifiedHeader(record.Name(), record.Tag()) {
						// Store them to process them later
						// since deletes only apply to processed headers we should at the end
						// only have the processed headers, if instead this header is added directly to 'headers'
						// it will contain duplicate headers for client/received and processed
						addProcessedHeaders(tempHeaders, record.Name(), record.Value())
					} else {
						// Received headers
						headers.Add(record.Name(), record.Value(), HdrStateReceived)
					}
				} else {
					addProcessedHeaders(headers, record.Name(), record.Value())
				}

			case HeaderUnsetRecord:
				var headers Headers
				if record.IsRespHeader() {
					headers = tx.RespHeaders()
				} else {
					headers = tx.ReqHeaders()
				}

				// all headers going forward now are considered as processed by VCL
				if clientHeaders {
					if isVarnishModifiedHeader(record.Name(), record.Tag()) {
						// Unset found while expecting client headers, assume we're on Varnish C code
						// add that header to a tempHeaders struct and parse it when the first VCL_call is encountered
						tempHeaders.Add(record.Name(), record.Value(), HdrStateReceived)
					} else {
						fmt.Printf("WARNING: unset found for non-tracked Varnish C code modificable header: %s\n", record.Name())
					}
				}
				headers.Delete(record.Name())
				tempHeaders.Delete(record.Name()) // Received headers are not deleted
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

// isVarnishModifiedHeader checks if a header is known to be modified
// or managed internally by Varnish in its C code.
//
// This includes headers like X-Forwarded-For, Via, and others
// that Varnish may add, remove, or alter during request/response handling.
func isVarnishModifiedHeader(name, tagName string) bool {
	if name == "" {
		return false
	}

	// Only consider Recv headers
	switch tagName {
	case tag.ReqHeader, tag.ReqUnset:
	default:
		return false
	}

	switch CanonicalHeaderName(name) {
	case "Via",
		"X-Forwarded-For",
		"X-Varnish",
		"Age",
		"Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"TE",
		"Trailer",
		"Transfer-Encoding",
		"Upgrade":
		return true
	default:
		return false
	}
}

// mergeTempHeaders is a helper function that should be called the first time clientHeaders is set to true
// it adds all the headers that have not been deleted, at the end it should only contain real client headers
func mergeTempHeaders(headers, tempHeaders Headers) {
	// Example: 'Via' header with value 'a' was sent by the client:
	//
	// -   ReqMethod      GET
	// -   ReqURL         /
	// -   ReqProtocol    HTTP/1.1
	// -   ReqHeader      Host: localhost:8001
	// -   ReqHeader      User-Agent: curl/8.7.1
	// -   ReqHeader      Accept: */*
	// -   ReqHeader      Via: a
	// -   ReqHeader      X-Forwarded-For: 192.168.65.1
	// -   ReqUnset       Via: a
	// -   ReqHeader      Via: a, 1.1 53d4be3da396 (Varnish/7.5)
	// -   VCL_call       RECV

	for name, h := range tempHeaders {
		for _, v := range h.Values(true) {
			if v.State() == HdrStateReceived {
				headers.Add(name, v.Value(), HdrStateReceived)
			}
		}
		for _, v := range h.Values(false) {
			if v.State() != HdrStateDeleted {
				headers.Add(name, v.Value(), v.State())
			} else {
				headers.Delete(name)
			}
		}
	}
	tempHeaders.Clear()
}

// addProcessedHeaders is a helper function to add headers processed in VCL or C code in Varnish
func addProcessedHeaders(headers Headers, name, value string) {
	if headers.Get(name, false) == "" {
		// Header does not exist, mark it as added
		headers.Add(name, value, HdrStateAdded)
	} else {
		// Header exist, add it as modified, VCL 'set' and 'unset' remove
		// all the previous values
		headers.Add(name, value, HdrStateModified)
	}
}
