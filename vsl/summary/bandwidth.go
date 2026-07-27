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

// BandwidthReport summarizes byte accounting (ReqAcct/BereqAcct) across a
// transaction set, split between client-facing traffic (client Request
// transactions) and backend-facing traffic (BeReq transactions).
//
// Piped transactions (PipeAcct) tunnel the same bytes over both the client
// and backend leg at once, so they don't split the same way and are left
// out of this report.
type BandwidthReport struct {
	Client  BandwidthTotals
	Backend BandwidthTotals
	Rows    []BandwidthRow
}

func Bandwidth(ts vsl.TransactionSet) BandwidthReport {
	var report BandwidthReport

	for _, tx := range ts.Transactions() {
		for _, r := range tx.Records {
			acct, ok := r.(vsl.AcctRecord)
			if !ok {
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
