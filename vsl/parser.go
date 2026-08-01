// SPDX-License-Identifier: MIT

package vsl

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"

	"github.com/aorith/varnishlog-parser/vsl/tags"
)

type TransactionParser struct {
	scanner *bufio.Scanner
}

const maxScanTokenSize = 4 * 1024 * 1024 // 4 MiB per line

// NewTransactionParser creates a new transaction parser from an 'io.Reader'
// that will feed varnishlog log lines.
func NewTransactionParser(r io.Reader) *TransactionParser {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), maxScanTokenSize)

	return &TransactionParser{
		scanner: sc,
	}
}

// Parse builds a varnishlog TransactionSet.
func (p *TransactionParser) Parse() (TransactionSet, error) {
	records, headerRawLog, err := p.tokenize()
	if err != nil {
		return TransactionSet{}, err
	}

	return buildTransactions(records, headerRawLog)
}

// isUint reports whether s is a non-empty run of decimal digits.
func isUint(s string) bool {
	if s == "" {
		return false
	}

	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}

	return true
}

// tokenize (pass 1) reads every line of the input and returns an ordered
// slice of BaseRecords, each stamped with the VXID of the transaction it
// belongs to, plus the raw header line text seen for each VXID (grouped
// mode only, used purely for display - see TransactionSet.RawLog).
//
// Line shapes are distinguished by their first field(s):
//
//   - A grouped-mode header, e.g.
//
//     "*   << Session  >> 16812342"
//     "**  << Request  >> 4"
//
//     not a record itself, it only establishes the VXID for the indented
//     tag lines that follow, until the next header.
//
//   - A flat "-g raw" tag line, e.g.
//
//     "1 Begin  c sess 0 HTTP/1"
//
//     the first field is a plain VXID, present on every line; the third
//     field is a client/backend/none marker, consumed here but ignored.
//
//   - A verbose ("-v") grouped-mode tag line, e.g.
//
//     "-   2 Begin  c req 1 rxreq"
//
//     same as a plain grouped-mode tag line, but with a VXID and marker
//     spliced in right after the depth marker, same as "-g raw" above.
//
//   - Anything else is a plain grouped-mode tag line, e.g.
//
//     "-   Begin  sess 0 HTTP/1"
func (p *TransactionParser) tokenize() ([]BaseRecord, map[VXID]string, error) {
	var records []BaseRecord // nolint:prealloc

	headerRawLog := make(map[VXID]string)

	var currentVXID VXID

	for p.scanner.Scan() {
		line := strings.TrimSpace(p.scanner.Text())
		fields := strings.Fields(line)

		if len(fields) < 2 {
			continue
		}

		switch {
		case fields[0][0] == '*':
			// Grouped-mode header, e.g:
			// *   << Session  >> 16812342
			// **  << Request  >> 4
			if len(fields) != 5 || fields[1][0] != '<' {
				continue
			}

			vxid, err := parseVXID(fields[4])
			if err != nil {
				return nil, nil, fmt.Errorf("incorrect vxid found on line %q, error: %w", line, err)
			}

			currentVXID = vxid
			headerRawLog[vxid] = line

		case isUint(fields[0]):
			// "-g raw" tag line: VXID first, no depth marker.
			blr, err := newVXIDTaggedBaseRecord(line, fields, 0)
			if err != nil {
				continue
			}

			records = append(records, blr)

		case len(fields) > 1 && isUint(fields[1]):
			// Verbose ("-v") grouped-mode tag line: depth marker, then VXID.
			blr, err := newVXIDTaggedBaseRecord(line, fields, 1)
			if err != nil {
				return nil, nil, err
			}

			records = append(records, blr)

		default:
			blr, err := NewBaseRecord(line)
			if err != nil {
				return nil, nil, err
			}

			blr.TxVXID = currentVXID
			records = append(records, blr)
		}
	}

	return records, headerRawLog, p.scanner.Err()
}

// buildTransactions (pass 2) assembles a TransactionSet from a flat, ordered
// stream of BaseRecords.
func buildTransactions(records []BaseRecord, headerRawLog map[VXID]string) (TransactionSet, error) {
	ts := TransactionSet{
		txs: make(map[VXID]*Transaction),
	}

	open := make(map[VXID]*Transaction)
	trackers := make(map[VXID]*headerTracker)

	for _, blr := range records {
		vxid := blr.TxVXID

		if vxid == 0 {
			// Non-transactional records (CLI, Backend_health, Witness,
			// WorkThread, ...), only surfaced by "-g raw".
			continue
		}

		r, err := processRecord(blr)
		if err != nil {
			return ts, err
		}

		if br, ok := r.(BeginRecord); ok {
			if _, exists := open[vxid]; exists {
				return ts, fmt.Errorf("parser error: duplicate %q tag found in the middle of transaction %d", tags.Begin, vxid)
			}

			tx := &Transaction{
				VXID:         vxid,
				TXType:       txTypeFromRecordType(br.RecordType),
				Parent:       br.Parent,
				ESILevel:     br.ESILevel,
				Reason:       br.Reason,
				TXID:         parseTXID(vxid, br.RecordType, br.Reason, br.ESILevel),
				RawLogHeader: headerRawLog[vxid],
				Records:      []Record{br},
				ReqHeaders:   make(map[string]Header),
				RespHeaders:  make(map[string]Header),
			}

			open[vxid] = tx
			trackers[vxid] = newHeaderTracker(tx.ReqHeaders, tx.RespHeaders)

			continue
		}

		tx, ok := open[vxid]
		if !ok {
			return ts, fmt.Errorf("parser error: %s tag found for vxid %d with no open transaction", r.GetTag(), vxid)
		}

		tx.Records = append(tx.Records, r)
		trackers[vxid].observe(r)

		if link, ok := r.(LinkRecord); ok {
			if slices.Contains(tx.Children, link.VXID) {
				slog.Warn("Parse() duplicate children assignment", "txid", tx.TXID, "linkTXID", link.TXID)
			} else {
				tx.Children = append(tx.Children, link.VXID)
			}
		}

		if r.GetTag() == tags.End {
			ts.txs[vxid] = tx
			delete(open, vxid)
			delete(trackers, vxid)
		}
	}

	// Here, 'open' might still contain transactions that were cut off mid-stream
	// rather than corrupt, expected when capturing a "-g raw" window.
	// Ignoring those partial txs to avoid having to deal with them.
	if len(open) > 0 {
		slog.Warn("buildTransactions() ended with some open (partial) transactions, those are ignored.", "open", len(open))
	}

	return ts, nil
}

// processRecord builds the appropriate dedicated Record type for blr's tag,
// falling back to the untyped BaseRecord itself for tags without one.
func processRecord(blr BaseRecord) (Record, error) {
	t := blr.GetTag()

	switch t { // nolint:revive
	case tags.End:
		return EndRecord{BaseRecord: blr}, nil
	case tags.RespReason, tags.BerespReason, tags.ObjReason:
		return ReasonRecord{BaseRecord: blr}, nil
	case tags.FetchError:
		return FetchErrorRecord{BaseRecord: blr}, nil
	case tags.Begin:
		return NewBeginRecord(blr)
	case tags.Link:
		return NewLinkRecord(blr)

		// Headers
	case tags.ReqHeader, tags.RespHeader, tags.BereqHeader, tags.BerespHeader, tags.ObjHeader:
		return NewHeaderRecord(blr)
	case tags.ObjUnset, tags.ReqUnset, tags.RespUnset, tags.BereqUnset, tags.BerespUnset:
		return NewHeaderUnsetRecord(blr)

	case tags.ReqMethod, tags.BereqMethod:
		return MethodRecord{BaseRecord: blr}, nil
	case tags.ReqProtocol, tags.RespProtocol, tags.BereqProtocol, tags.BerespProtocol, tags.ObjProtocol:
		return ProtocolRecord{BaseRecord: blr}, nil
	case tags.BackendOpen:
		return NewBackendOpenRecord(blr)
	case tags.BackendStart:
		return NewBackendStartRecord(blr)
	case tags.BackendClose:
		return NewBackendCloseRecord(blr)
	case tags.BackendReuse:
		return NewBackendReuseRecord(blr)
	case tags.Brotli:
		return NewBrotliRecord(blr)
	case tags.ReqAcct, tags.BereqAcct:
		return NewAcctRecord(blr)
	case tags.PipeAcct:
		return NewPipeAcctRecord(blr)
	case tags.Timestamp:
		return NewTimestampRecord(blr)
	case tags.ReqStart:
		return NewReqStartRecord(blr)
	case tags.ReqURL, tags.BereqURL:
		return NewURLRecord(blr)
	case tags.Filters:
		return NewFiltersRecord(blr)
	case tags.RespStatus, tags.BerespStatus, tags.ObjStatus:
		return NewStatusRecord(blr)
	case tags.Length:
		return NewLengthRecord(blr)
	case tags.MSE4NewObject:
		return NewMSE4NewObjectRecord(blr)
	case tags.MSE4ObjIter:
		return NewMSE4ObjIterRecord(blr)
	case tags.MSE4ChunkFault:
		return NewMSE4ChunkFaultRecord(blr)
	case tags.Hit, tags.HitMiss, tags.HitPass:
		return NewHitRecord(blr)
	case tags.TTL:
		return NewTTLRecord(blr)
	case tags.VCLLog:
		return NewVCLLogRecord(blr)
	case tags.Storage:
		return NewStorageRecord(blr)
	case tags.FetchBody:
		return NewFetchBodyRecord(blr)
	case tags.SessOpen:
		return NewSessOpenRecord(blr)
	case tags.SessClose:
		return NewSessCloseRecord(blr)
	case tags.Gzip:
		return NewGzipRecord(blr)
	case tags.VCLCall:
		return VCLCallRecord{BaseRecord: blr}, nil
	case tags.VCLReturn:
		return VCLReturnRecord{BaseRecord: blr}, nil
	case tags.VCLUse:
		return VCLUseRecord{BaseRecord: blr}, nil
	case tags.Error:
		return ErrorRecord{BaseRecord: blr}, nil
	case tags.XBody, tags.YKEY, tags.BogoHeader, tags.HTTPGarbage, tags.ESIXMLError,
		tags.LostHeader, tags.Proxy, tags.ProxyGarbage, tags.VCLAcl, tags.VCLError,
		tags.VCLTrace, tags.Notice, tags.VfpAcct,
		tags.ADNS, tags.Backend, tags.BackendSSL, tags.Body, tags.ConnectAcct,
		tags.Crypto, tags.DataDome, tags.Debug, tags.Edgestash,
		tags.H2RxBody, tags.H2RxHdr, tags.H2TxBody, tags.H2TxHdr, tags.Hash,
		tags.MSE4Eviction, tags.MSE4YKEYIter, tags.Nodes, tags.OCSP, tags.OCSPError,
		tags.ReqTarget, tags.TLS, tags.VHA6, tags.WAF:
		// tags without a dedicated struct
		return blr, nil
	case tags.ExpBan, tags.ExpKill, tags.VSL, tags.SessError,
		tags.CLI, tags.BackendHealth, tags.Witness, tags.WorkThread:
		// Non-transactional tags, only ever seen under VXID 0 in "-g raw".
		return blr, nil
	default:
		slog.Warn("unknown tag", "tag", t)

		return blr, nil
	}
}
