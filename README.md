# varnishlog-parser

> Varnishlog Parser is a small Go library built to parse and analyze `varnishlog`
> output, just like the name suggests.

A frontend to easily parse the logs is implemented using this library.

An instance is available here: [varnishlog.iou.re](https://varnishlog.iou.re/)

## Use as a library

Check the reference [documentation](https://pkg.go.dev/github.com/aorith/varnishlog-parser)

```go
package main

import (
	"encoding/json"
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

	// Iterate all the transactions VSL log records
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

Transactions can be marshaled into JSON:

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
    "Level": 1,
    "Reason": "rxreq",
    "ESILevel": 0,
    "TXType": "Request",
    "RawLog": "*   \u003c\u003c Request  \u003e\u003e 4",
    "Records": [
      {
        "Tag": "Begin",
        "RawValue": "req 1 rxreq",
        "RecordType": "req",
        "Parent": 1,
        "ESILevel": 0,
        "Reason": "rxreq"
      },
      {
        "Tag": "Timestamp",
        "RawValue": "Start: 1763030681.497130 0.000000 0.000000",
        "EventLabel": "Start",
        "StartTime": "2025-11-13T11:44:41.49713+01:00",
        "AbsoluteTime": "2025-11-13T11:44:41.49713+01:00",
        "SinceStart": 0,
        "SinceLast": 0
      },
      {
        "Tag": "Timestamp",
        "RawValue": "Req: 1763030681.497130 0.000000 0.000000",
        "EventLabel": "Req",
        "StartTime": "2025-11-13T11:44:41.49713+01:00",
        "AbsoluteTime": "2025-11-13T11:44:41.49713+01:00",
        "SinceStart": 0,
        "SinceLast": 0
      },
      {
        "Tag": "VCL_use",
        "RawValue": "boot"
      },
      {
        "Tag": "ReqStart",
        "RawValue": "192.168.65.1 54660 http",
        "ClientIP": "192.168.65.1",
        "ClientPort": 54660,
        "Listener": "http"
      },

      [ . . . ]

      {
        "Tag": "RespHeader",
        "RawValue": "Age: 0",
        "Name": "Age",
        "Value": "0",
        "HeaderType": "RespHeader"
      },
      {
        "Tag": "RespHeader",
        "RawValue": "Via: 1.1 e088e52945df (Varnish/7.7)",
        "Name": "Via",
        "Value": "1.1 e088e52945df (Varnish/7.7)",
        "HeaderType": "RespHeader"
      },
    ],
    "ReqHeaders": { ... },
    "RespHeaders": { ... },
    "Parent": 1,
    "Children": null
  }
]
```

## Run the web-ui locally

Either clone this repository and run:

```sh
go run cmd/server
```

Or with docker/podman:

```sh
docker run --rm -p 8080:8080 ghcr.io/aorith/varnishlog-parser:latest
```
