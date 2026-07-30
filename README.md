# varnishlog-parser

> Varnishlog Parser is a small Go library built to parse and analyze `varnishlog`
> output, just like the name suggests.

A frontend to easily parse the logs is implemented using this library.

An instance is available here: [varnishlog.iou.re](https://varnishlog.iou.re/)

## Install

```sh
go get github.com/aorith/varnishlog-parser
```

Reference [documentation](https://pkg.go.dev/github.com/aorith/varnishlog-parser) is available on pkg.go.dev.

## Quick start

```go
package main

import (
	"fmt"
	"strings"

	"github.com/aorith/varnishlog-parser/assets"
	"github.com/aorith/varnishlog-parser/vsl"
)

func main() {
	p := vsl.NewTransactionParser(strings.NewReader(assets.VCLCached)) // Replace with your own varnishlog log
	txsSet, err := p.Parse()
	if err != nil {
		fmt.Println(err)
		return
	}

	// Iterate all the transactions and their VSL log records
	for _, tx := range txsSet.Transactions() {
		fmt.Printf("%v\n", tx.TXID)
		for _, r := range tx.Records {
			switch record := r.(type) {
			case vsl.TimestampRecord:
				fmt.Printf("  [%s]  %s: %s\n", record.GetTag(), record.EventLabel, record.SinceLast.String())
			case vsl.SessCloseRecord:
				fmt.Printf("  [%s]  %s: %s\n", record.GetTag(), record.Reason, record.Duration.String())
			default:
				fmt.Printf("  [%s]  %s\n", record.GetTag(), record.GetRawValue())
			}
		}
	}
}
```

Output:

```
6-sess
  [Begin]  sess 0 HTTP/1
  [SessOpen]  192.168.65.1 61200 http 192.168.50.10 80 1762612268.273411 28
  [Link]  req 9 rxreq
  [SessClose]  REM_CLOSE: 111ms
  [End]
9-req-rxreq
  [Begin]  req 6 rxreq
  [Timestamp]  Start: 0s
  [Timestamp]  Req: 0s
  [VCL_use]  boot
  [ReqStart]  192.168.65.1 61200 http
  [ReqMethod]  GET
  [ReqURL]  /item
  [ReqProtocol]  HTTP/1.1

  [ . . . ]

  [RespHeader]  X-Varnish: 9 8
  [RespHeader]  Age: 0
  [RespHeader]  Via: 1.1 4dab8a10025c (Varnish/7.7)
  [RespHeader]  Accept-Ranges: bytes
  [VCL_call]  DELIVER
  [VCL_return]  deliver
  [Timestamp]  Process: 100µs
  [Filters]
  [RespHeader]  Connection: keep-alive
  [Timestamp]  Resp: 126µs
  [ReqAcct]  121 0 121 251 304 555
  [End]
```

## Useful types and methods

Parsing always starts with `vsl.NewTransactionParser(r io.Reader).Parse()`, which
returns a `vsl.TransactionSet`: a collection of every `Begin`/`End` transaction
found in the log (sessions, client requests and backend requests), linked
together by their parent/child relationships.

`vsl.TransactionSet`:

- `Transactions() []*Transaction` - every parsed transaction, sorted by VXID.
- `GetTX(vxid VXID) *Transaction` / `GetChildTX(parent, child VXID) *Transaction` - direct lookups.
- `UniqueRootParents(includeSession bool) []*Transaction` - only the top-level transactions, e.g. one entry per logical client request instead of every session/request/bereq/ESI sub-transaction. Usually the best starting point when iterating "requests" rather than raw transactions.
- `RawLog() string` / `RawLogForTx(tx *Transaction, includeChildren bool) string` - reconstruct the original VSL text for the whole set, or for a single transaction (optionally with its children).

`vsl.Transaction`:

- `RecordByTag(tag string, first bool) Record` - the first or last record matching a VSL tag (see the `vsl/tags` package for tag constants), e.g. `tx.RecordByTag(tags.Hit, true)`.
- `RecordValueByTag(tag string, first bool) string` - same, but returns the raw value directly.
- `LastRecordByTag` / `NextRecordByTag` - search backwards/forwards from a given index, useful while walking `tx.Records` in order.
- `Duration()`, `StartTime()`, `EndTime()` - approximate transaction timing.
- `ReqHeaders` / `RespHeaders Headers` - final, post-VCL header state. `Headers.Get(name string, received bool)` fetches a header value; pass `received: true` to get the original value as sent by the client/backend, before any VCL rewriting.

## Reports and diagnostics

Two packages build on top of a `vsl.TransactionSet` to summarize or analyze it:

- **`vsl/summary`**
  - `Bandwidth(ts) BandwidthReport` - byte accounting (`ReqAcct`/`BereqAcct`) split between client and backend traffic, plus a per-transaction breakdown.
  - `TimestampEventsSummary(ts) []*LatencyCounter` - min/max/avg/p90/p99 latency for every VSL `Timestamp` event label (e.g. `Fetch`, `Process`, `Resp`).
- **`vsl/diagnostics`**
  - `Run(ts) []Finding` - scans every transaction for common Varnish misconfigurations and errors: cache fragmentation (`Vary: User-Agent`, `Vary: *`, ...), a cached `Set-Cookie` or `Authorization` response served on a hit, long hit-for-pass/grace periods, backend/ESI/VCL errors, malformed requests, abnormal session closes, retry storms, and more. Each `Finding` carries a `Severity` (Info/Warning/Critical), a stable `Rule` id, a human-readable summary/detail, and the offending transaction's `TXID`/`VXID`.

```go
for _, finding := range diagnostics.Run(txsSet) {
	fmt.Println(finding) // [Critical] vary-user-agent (10-req-rxreq): Vary: User-Agent fragments the cache... - Vary header: "User-Agent, Accept-Encoding"
}
```

This is the same engine used to power the "Diagnostics" section of the web UI.

## Marshaling to JSON

Transactions can be marshaled into JSON directly:

```go
b, err := json.MarshalIndent(txsSet.Transactions(), "", "  ")
if err != nil {
	panic(err)
}
fmt.Println(string(b))
```

Output (trimmed for brevity):

```json
[
  {
    "TXID": "4-req-rxreq",
    "VXID": 4,
    "TXType": "Request",
    "Records": [
      {
        "Tag": "Begin",
        "RawValue": "req 1 rxreq",
        "RecordType": "req",
        "Parent": 1,
        "Reason": "rxreq"
      },
      {
        "Tag": "RespHeader",
        "RawValue": "Via: 1.1 e088e52945df (Varnish/7.7)",
        "Name": "Via",
        "Value": "1.1 e088e52945df (Varnish/7.7)",
        "HeaderType": "RespHeader"
      }
    ],
    "ReqHeaders": { ... },
    "RespHeaders": { ... },
    "Parent": 1,
    "Children": null
  }
]
```

## Run the web UI locally

Either clone this repository and run:

```sh
go run cmd/server
```

Or with docker/podman:

```sh
docker run --rm -p 8080:8080 ghcr.io/aorith/varnishlog-parser:latest
```
