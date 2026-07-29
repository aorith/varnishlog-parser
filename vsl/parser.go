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

func NewTransactionParser(r io.Reader) *TransactionParser {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), maxScanTokenSize)

	return &TransactionParser{
		scanner: sc,
	}
}

func (p *TransactionParser) Parse() (TransactionSet, error) {
	ts := TransactionSet{
		txs: make(map[VXID]*Transaction),
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
			return ts, err
		}

		// Expect a Begin tag after the start of the transaction, eg:
		// --- Begin          req 2 esi 1
		if !p.scanner.Scan() {
			return ts, fmt.Errorf("parser error: expected %s tag, found EOF after %q", tags.Begin, tx.RawLog)
		}

		line = strings.TrimSpace(p.scanner.Text())
		if line == "" {
			return ts, fmt.Errorf("parser error: expected %s tag, found empty line after %q", tags.Begin, tx.RawLog)
		}

		r, err := processRecord(line)
		if err != nil {
			return ts, err
		}

		if r.GetTag() != tags.Begin {
			return ts, fmt.Errorf("parser error: expected %s tag, found %q on line %q", tags.Begin, r.GetTag(), line)
		}

		// Add the data contained in the Begin tag to the new transaction
		br := r.(BeginRecord) // nolint
		tx.Parent = br.Parent
		tx.ESILevel = br.ESILevel
		tx.TXID = parseTXID(tx.VXID, br.RecordType, br.Reason, br.ESILevel)
		tx.Reason = br.Reason
		tx.Records = append(tx.Records, br)

		// Parse the remaining tags
		complete := false // to check at the end if the transaction finished (found End tag for example)
		ht := newHeaderTracker(tx.ReqHeaders, tx.RespHeaders)

		for p.scanner.Scan() {
			line := strings.TrimSpace(p.scanner.Text())
			// Skip empty lines or invalid lines
			if len(strings.Fields(line)) < 2 {
				continue
			}

			r, err := processRecord(line)
			if err != nil {
				return ts, err
			}

			tx.Records = append(tx.Records, r)
			ht.observe(r)

			switch record := r.(type) {
			case LinkRecord:
				if slices.Contains(tx.Children, record.VXID) {
					slog.Warn("Parse() duplicate children assignment", "txid", tx.TXID, "linkTXID", record.TXID)

					continue
				}

				tx.Children = append(tx.Children, record.VXID)

			case BeginRecord:
				// A Begin tag was found in the middle of a transaction
				return ts, fmt.Errorf("parser error: duplicate %q tag found in the middle of transaction %d", tags.Begin, tx.VXID)

			default:
			}

			// Check if the tx is complete, this is outside of the switch case to be able to break the for loop
			if r.GetTag() == tags.End {
				ts.txs[tx.VXID] = tx
				complete = true

				break
			}
		}

		err = p.scanner.Err()
		if err != nil {
			return ts, err
		}

		if !complete {
			return ts, fmt.Errorf("parser error: transaction %q finished without %s tag at EOL", tx.RawLog, tags.End)
		}
	}

	err := p.scanner.Err()
	if err != nil {
		return ts, err
	}

	return ts, nil
}

func processRecord(line string) (Record, error) {
	blr, err := NewBaseRecord(line)
	if err != nil {
		return blr, err
	}

	t := blr.GetTag()
	switch t {
	case tags.End:
		return EndRecord{BaseRecord: blr}, nil
	case tags.RespReason, tags.BerespReason:
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
	case tags.YKEY:
		// tags without a dedicated struct
		return blr, nil
	default:
		slog.Warn("unknown tag", "tag", t)

		return blr, nil
	}
}
