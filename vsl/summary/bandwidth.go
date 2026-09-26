// SPDX-License-Identifier: MIT

package summary

import (
	"cmp"
	"slices"

	"github.com/aorith/varnishlog-parser/vsl"
)

// BandwidthTotals holds summed byte accounting (see vsl.AcctRecord) for a
// group of transactions.
type BandwidthTotals struct {
	HeaderTx vsl.SizeValue
	BodyTx   vsl.SizeValue
	TotalTx  vsl.SizeValue
	HeaderRx vsl.SizeValue
	BodyRx   vsl.SizeValue
	TotalRx  vsl.SizeValue
}

func (t *BandwidthTotals) add(a vsl.AcctRecord) {
	t.HeaderTx += a.HeaderTx
	t.BodyTx += a.BodyTx
	t.TotalTx += a.TotalTx
	t.HeaderRx += a.HeaderRx
	t.BodyRx += a.BodyRx
	t.TotalRx += a.TotalRx
}

// BandwidthRow is a single transaction's own byte accounting.
type BandwidthRow struct {
	BandwidthTotals

	TXID   vsl.TXID
	TXType vsl.TxType
}

// BandwidthReport summarizes byte accounting (ReqAcct/BereqAcct/PipeAcct)
// across a transaction set, split between client-facing traffic (client
// Request transactions) and backend-facing traffic (BeReq transactions).
//
// A piped request logs PipeAcct on the client transaction and an all-zero
// BereqAcct on its BeReq, so the backend leg is taken from the parent's
// PipeAcct instead. Piped bytes towards the client include the raw backend
// response headers, so they only count towards TotalTx/TotalRx.
type BandwidthReport struct {
	Client  BandwidthTotals
	Backend BandwidthTotals
	Rows    []BandwidthRow
}

func Bandwidth(ts vsl.TransactionSet) BandwidthReport {
	var report BandwidthReport

	for _, tx := range ts.Transactions() {
		for _, r := range tx.Records {
			var acct vsl.AcctRecord

			switch r := r.(type) {
			case vsl.AcctRecord:
				acct = r

				if tx.TXType == vsl.TxTypeBereq {
					if pa, ok := pipeAcct(ts.GetTX(tx.Parent)); ok {
						acct = vsl.AcctRecord{
							HeaderTx: pa.BackendReqHeaders,
							BodyTx:   pa.PipedFrom,
							TotalTx:  pa.BackendReqHeaders + pa.PipedFrom,
							TotalRx:  pa.PipedTo,
						}
					}
				}
			case vsl.PipeAcctRecord:
				acct = vsl.AcctRecord{
					HeaderRx: r.ClientReqHeaders,
					BodyRx:   r.PipedFrom,
					TotalRx:  r.ClientReqHeaders + r.PipedFrom,
					TotalTx:  r.PipedTo,
				}
			default:
				continue
			}

			row := BandwidthRow{TXID: tx.TXID, TXType: tx.TXType}
			row.add(acct)
			report.Rows = append(report.Rows, row)

			if tx.TXType == vsl.TxTypeRequest {
				report.Client.add(acct)
			} else {
				report.Backend.add(acct)
			}
		}
	}

	slices.SortFunc(report.Rows, func(a, b BandwidthRow) int {
		return cmp.Compare(b.TotalTx+b.TotalRx, a.TotalTx+a.TotalRx)
	})

	return report
}

func pipeAcct(tx *vsl.Transaction) (vsl.PipeAcctRecord, bool) {
	if tx == nil {
		return vsl.PipeAcctRecord{}, false
	}

	for _, r := range tx.Records {
		if pa, ok := r.(vsl.PipeAcctRecord); ok {
			return pa, true
		}
	}

	return vsl.PipeAcctRecord{}, false
}
