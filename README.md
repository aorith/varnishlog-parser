# varnishlog-parser

> Varnishlog Parser is a small Go library built to parse and analyze `varnishlog`
> output, just like the name suggests.
> It doesn’t rely on any external Go dependencies.

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

	// Marshal into JSON
	b, err := json.MarshalIndent(txsSet.Transactions(), "", "  ")
	if err != nil {
		panic(err)
	}
	fmt.Println(string(b))
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
  [ReqHeader]  Host: varnishlog.iou.re
  [ReqHeader]  Accept: */*
  [ReqHeader]  X-Forwarded-For: 1.2.3.4
  [ReqHeader]  Cached: 1
  [ReqHeader]  User-Agent: hurl/7.0.0
  [ReqUnset]  X-Forwarded-For: 1.2.3.4
  [ReqHeader]  X-Forwarded-For: 1.2.3.4, 192.168.65.1
  [ReqHeader]  Via: 1.1 4dab8a10025c (Varnish/7.7)
  [VCL_call]  RECV
  [ReqHeader]  whoami: 1
  [VCL_return]  hash
  [VCL_call]  HASH
  [VCL_return]  lookup
  [Hit]  8 4.892716 10.000000 0.000000
  [VCL_call]  HIT
  [VCL_return]  deliver
  [RespProtocol]  HTTP/1.1
  [RespStatus]  200
  [RespReason]  OK
  [RespHeader]  Date: Sat, 08 Nov 2025 14:31:08 GMT
  [RespHeader]  Content-Length: 304
  [RespHeader]  Content-Type: text/plain; charset=utf-8
  [RespHeader]  Cache-Control: max-age=5
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

Output:

```json
[
  {
    "TXID": "6-sess",
    "VXID": 6,
    "Level": 1,
    "ESILevel": 0,
    "TXType": "Session",
    "RawLog": "*   \u003c\u003c Session  \u003e\u003e 6",
    "Records": [
      {
        "Tag": "Begin",
        "RawValue": "sess 0 HTTP/1",
        "RecordType": "sess",
        "Parent": 0,
        "ESILevel": 0,
        "Reason": "HTTP/1"
      },
      {
        "Tag": "SessOpen",
        "RawValue": "192.168.65.1 61200 http 192.168.50.10 80 1762612268.273411 28",
        "RemoteAddr": "192.168.65.1",
        "RemotePort": 61200,
        "SocketName": "http",
        "LocalAddr": "192.168.50.10",
        "LocalPort": 80,
        "SessionStart": "2025-11-08T15:31:08.273411+01:00",
        "FileDescriptor": 28
      },
      {
        "Tag": "Link",
        "RawValue": "req 9 rxreq",
        "TXID": "9-req-rxreq",
        "VXID": 9,
        "TXType": "req",
        "Reason": "rxreq",
        "ESILevel": 0
      },
      {
        "Tag": "SessClose",
        "RawValue": "REM_CLOSE 0.111",
        "Reason": "REM_CLOSE",
        "Duration": 111000000
      },
      {
        "Tag": "End",
        "RawValue": ""
      }
    ],
    "ReqHeaders": {},
    "RespHeaders": {},
    "Parent": 0,
    "Children": [9]
  },
  {
    "TXID": "9-req-rxreq",
    "VXID": 9,
    "Level": 2,
    "ESILevel": 0,
    "TXType": "Request",
    "RawLog": "**  \u003c\u003c Request  \u003e\u003e 9",
    "Records": [
      {
        "Tag": "Begin",
        "RawValue": "req 6 rxreq",
        "RecordType": "req",
        "Parent": 6,
        "ESILevel": 0,
        "Reason": "rxreq"
      },
      {
        "Tag": "Timestamp",
        "RawValue": "Start: 1762612268.383000 0.000000 0.000000",
        "EventLabel": "Start",
        "StartTime": "2025-11-08T15:31:08.383+01:00",
        "AbsoluteTime": "2025-11-08T15:31:08.383+01:00",
        "SinceStart": 0,
        "SinceLast": 0
      },
      {
        "Tag": "Timestamp",
        "RawValue": "Req: 1762612268.383000 0.000000 0.000000",
        "EventLabel": "Req",
        "StartTime": "2025-11-08T15:31:08.383+01:00",
        "AbsoluteTime": "2025-11-08T15:31:08.383+01:00",
        "SinceStart": 0,
        "SinceLast": 0
      },
      {
        "Tag": "VCL_use",
        "RawValue": "boot"
      },
      {
        "Tag": "ReqStart",
        "RawValue": "192.168.65.1 61200 http",
        "ClientIP": "192.168.65.1",
        "ClientPort": 61200,
        "Listener": "http"
      },
      {
        "Tag": "ReqMethod",
        "RawValue": "GET"
      },
      {
        "Tag": "ReqURL",
        "RawValue": "/item",
        "Path": "/item",
        "QueryString": ""
      },
      {
        "Tag": "ReqProtocol",
        "RawValue": "HTTP/1.1"
      },
      {
        "Tag": "ReqHeader",
        "RawValue": "Host: varnishlog.iou.re",
        "Name": "Host",
        "Value": "varnishlog.iou.re",
        "HeaderType": "ReqHeader"
      },
      {
        "Tag": "ReqHeader",
        "RawValue": "Accept: */*",
        "Name": "Accept",
        "Value": "*/*",
        "HeaderType": "ReqHeader"
      },
      {
        "Tag": "ReqHeader",
        "RawValue": "X-Forwarded-For: 1.2.3.4",
        "Name": "X-Forwarded-For",
        "Value": "1.2.3.4",
        "HeaderType": "ReqHeader"
      },
      {
        "Tag": "ReqHeader",
        "RawValue": "Cached: 1",
        "Name": "Cached",
        "Value": "1",
        "HeaderType": "ReqHeader"
      },
      {
        "Tag": "ReqHeader",
        "RawValue": "User-Agent: hurl/7.0.0",
        "Name": "User-Agent",
        "Value": "hurl/7.0.0",
        "HeaderType": "ReqHeader"
      },
      {
        "Tag": "ReqUnset",
        "RawValue": "X-Forwarded-For: 1.2.3.4",
        "Name": "X-Forwarded-For",
        "Value": "1.2.3.4",
        "HeaderType": "ReqUnset"
      },
      {
        "Tag": "ReqHeader",
        "RawValue": "X-Forwarded-For: 1.2.3.4, 192.168.65.1",
        "Name": "X-Forwarded-For",
        "Value": "1.2.3.4, 192.168.65.1",
        "HeaderType": "ReqHeader"
      },
      {
        "Tag": "ReqHeader",
        "RawValue": "Via: 1.1 4dab8a10025c (Varnish/7.7)",
        "Name": "Via",
        "Value": "1.1 4dab8a10025c (Varnish/7.7)",
        "HeaderType": "ReqHeader"
      },
      {
        "Tag": "VCL_call",
        "RawValue": "RECV"
      },
      {
        "Tag": "ReqHeader",
        "RawValue": "whoami: 1",
        "Name": "Whoami",
        "Value": "1",
        "HeaderType": "ReqHeader"
      },
      {
        "Tag": "VCL_return",
        "RawValue": "hash"
      },
      {
        "Tag": "VCL_call",
        "RawValue": "HASH"
      },
      {
        "Tag": "VCL_return",
        "RawValue": "lookup"
      },
      {
        "Tag": "Hit",
        "RawValue": "8 4.892716 10.000000 0.000000",
        "ObjVXID": 8,
        "TTL": 4892716000,
        "Grace": 10000000000,
        "Keep": 0
      },
      {
        "Tag": "VCL_call",
        "RawValue": "HIT"
      },
      {
        "Tag": "VCL_return",
        "RawValue": "deliver"
      },
      {
        "Tag": "RespProtocol",
        "RawValue": "HTTP/1.1"
      },
      {
        "Tag": "RespStatus",
        "RawValue": "200",
        "Status": 200
      },
      {
        "Tag": "RespReason",
        "RawValue": "OK"
      },
      {
        "Tag": "RespHeader",
        "RawValue": "Date: Sat, 08 Nov 2025 14:31:08 GMT",
        "Name": "Date",
        "Value": "Sat, 08 Nov 2025 14:31:08 GMT",
        "HeaderType": "RespHeader"
      },
      {
        "Tag": "RespHeader",
        "RawValue": "Content-Length: 304",
        "Name": "Content-Length",
        "Value": "304",
        "HeaderType": "RespHeader"
      },
      {
        "Tag": "RespHeader",
        "RawValue": "Content-Type: text/plain; charset=utf-8",
        "Name": "Content-Type",
        "Value": "text/plain; charset=utf-8",
        "HeaderType": "RespHeader"
      },
      {
        "Tag": "RespHeader",
        "RawValue": "Cache-Control: max-age=5",
        "Name": "Cache-Control",
        "Value": "max-age=5",
        "HeaderType": "RespHeader"
      },
      {
        "Tag": "RespHeader",
        "RawValue": "X-Varnish: 9 8",
        "Name": "X-Varnish",
        "Value": "9 8",
        "HeaderType": "RespHeader"
      },
      {
        "Tag": "RespHeader",
        "RawValue": "Age: 0",
        "Name": "Age",
        "Value": "0",
        "HeaderType": "RespHeader"
      },
      {
        "Tag": "RespHeader",
        "RawValue": "Via: 1.1 4dab8a10025c (Varnish/7.7)",
        "Name": "Via",
        "Value": "1.1 4dab8a10025c (Varnish/7.7)",
        "HeaderType": "RespHeader"
      },
      {
        "Tag": "RespHeader",
        "RawValue": "Accept-Ranges: bytes",
        "Name": "Accept-Ranges",
        "Value": "bytes",
        "HeaderType": "RespHeader"
      },
      {
        "Tag": "VCL_call",
        "RawValue": "DELIVER"
      },
      {
        "Tag": "VCL_return",
        "RawValue": "deliver"
      },
      {
        "Tag": "Timestamp",
        "RawValue": "Process: 1762612268.383100 0.000100 0.000100",
        "EventLabel": "Process",
        "StartTime": "2025-11-08T15:31:08.383+01:00",
        "AbsoluteTime": "2025-11-08T15:31:08.3831+01:00",
        "SinceStart": 100000,
        "SinceLast": 100000
      },
      {
        "Tag": "Filters",
        "RawValue": "",
        "Filters": []
      },
      {
        "Tag": "RespHeader",
        "RawValue": "Connection: keep-alive",
        "Name": "Connection",
        "Value": "keep-alive",
        "HeaderType": "RespHeader"
      },
      {
        "Tag": "Timestamp",
        "RawValue": "Resp: 1762612268.383226 0.000226 0.000126",
        "EventLabel": "Resp",
        "StartTime": "2025-11-08T15:31:08.3831+01:00",
        "AbsoluteTime": "2025-11-08T15:31:08.383226+01:00",
        "SinceStart": 226000,
        "SinceLast": 126000
      },
      {
        "Tag": "ReqAcct",
        "RawValue": "121 0 121 251 304 555",
        "HeaderTx": 121,
        "BodyTx": 0,
        "TotalTx": 121,
        "HeaderRx": 251,
        "BodyRx": 304,
        "TotalRx": 555
      },
      {
        "Tag": "End",
        "RawValue": ""
      }
    ],
    "ReqHeaders": {
      "Accept": {
        "Values": [
          {
            "Value": "*/*",
            "State": "Received"
          }
        ],
        "ReceivedValues": [
          {
            "Value": "*/*",
            "State": "Received"
          }
        ]
      },
      "Cached": {
        "Values": [
          {
            "Value": "1",
            "State": "Received"
          }
        ],
        "ReceivedValues": [
          {
            "Value": "1",
            "State": "Received"
          }
        ]
      },
      "Host": {
        "Values": [
          {
            "Value": "varnishlog.iou.re",
            "State": "Received"
          }
        ],
        "ReceivedValues": [
          {
            "Value": "varnishlog.iou.re",
            "State": "Received"
          }
        ]
      },
      "User-Agent": {
        "Values": [
          {
            "Value": "hurl/7.0.0",
            "State": "Received"
          }
        ],
        "ReceivedValues": [
          {
            "Value": "hurl/7.0.0",
            "State": "Received"
          }
        ]
      },
      "Via": {
        "Values": [
          {
            "Value": "1.1 4dab8a10025c (Varnish/7.7)",
            "State": "Added"
          }
        ],
        "ReceivedValues": []
      },
      "Whoami": {
        "Values": [
          {
            "Value": "1",
            "State": "Added"
          }
        ],
        "ReceivedValues": []
      },
      "X-Forwarded-For": {
        "Values": [
          {
            "Value": "1.2.3.4, 192.168.65.1",
            "State": "Modified"
          }
        ],
        "ReceivedValues": [
          {
            "Value": "1.2.3.4",
            "State": "Received"
          }
        ]
      }
    },
    "RespHeaders": {
      "Accept-Ranges": {
        "Values": [
          {
            "Value": "bytes",
            "State": "Received"
          }
        ],
        "ReceivedValues": [
          {
            "Value": "bytes",
            "State": "Received"
          }
        ]
      },
      "Age": {
        "Values": [
          {
            "Value": "0",
            "State": "Received"
          }
        ],
        "ReceivedValues": [
          {
            "Value": "0",
            "State": "Received"
          }
        ]
      },
      "Cache-Control": {
        "Values": [
          {
            "Value": "max-age=5",
            "State": "Received"
          }
        ],
        "ReceivedValues": [
          {
            "Value": "max-age=5",
            "State": "Received"
          }
        ]
      },
      "Connection": {
        "Values": [
          {
            "Value": "keep-alive",
            "State": "Added"
          }
        ],
        "ReceivedValues": []
      },
      "Content-Length": {
        "Values": [
          {
            "Value": "304",
            "State": "Received"
          }
        ],
        "ReceivedValues": [
          {
            "Value": "304",
            "State": "Received"
          }
        ]
      },
      "Content-Type": {
        "Values": [
          {
            "Value": "text/plain; charset=utf-8",
            "State": "Received"
          }
        ],
        "ReceivedValues": [
          {
            "Value": "text/plain; charset=utf-8",
            "State": "Received"
          }
        ]
      },
      "Date": {
        "Values": [
          {
            "Value": "Sat, 08 Nov 2025 14:31:08 GMT",
            "State": "Received"
          }
        ],
        "ReceivedValues": [
          {
            "Value": "Sat, 08 Nov 2025 14:31:08 GMT",
            "State": "Received"
          }
        ]
      },
      "Via": {
        "Values": [
          {
            "Value": "1.1 4dab8a10025c (Varnish/7.7)",
            "State": "Received"
          }
        ],
        "ReceivedValues": [
          {
            "Value": "1.1 4dab8a10025c (Varnish/7.7)",
            "State": "Received"
          }
        ]
      },
      "X-Varnish": {
        "Values": [
          {
            "Value": "9 8",
            "State": "Received"
          }
        ],
        "ReceivedValues": [
          {
            "Value": "9 8",
            "State": "Received"
          }
        ]
      }
    },
    "Parent": 6,
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
