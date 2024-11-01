# varnishlog-parser

> A varnishlog parser library and web user interface

An instance is available here: https://varnishlog.iou.re/

## Run the web-ui locally

Either clone this repository and run:

```sh
go run . server
```

Or with docker/podman:

```sh
docker run --rm -p 8080:8080 ghcr.io/aorith/varnishlog-parser:latest
```

## Use as a library

Here's an example:

```go
package main

import (
	"fmt"
	"strings"

	"github.com/aorith/varnishlog-parser/vsl"
)

const incompleteExample = `*   << Session  >> 1
-   Begin          sess 0 HTTP/1
-   SessOpen       192.168.50.1 55650 http 192.168.50.10 80 1728889150.256391 26
-   Link           req 2 rxreq
-   SessClose      REM_CLOSE 0.123
-   End
**  << Request  >> 2
--  Begin          req 1 rxreq
--  Timestamp      Start: 1728889150.256523 0.021300 0.021300
--  VCL_use        boot
--  End
`

func main() {
	p := vsl.NewTransactionParser(strings.NewReader(incompleteExample))
	txsSet, err := p.Parse()
	if err != nil {
		fmt.Println(err)
		return
	}

	// Iterate all the transactions VSL log records
	for _, tx := range txsSet.Transactions() {
		fmt.Printf("%v\n", tx.TXID())
		for _, r := range tx.LogRecords() {
			switch record := r.(type) {
			case vsl.TimestampRecord:
				fmt.Printf("  [%s]  %s: %s\n", record.Tag(), record.EventLabel(), record.SinceLast().String())
			case vsl.SessCloseRecord:
				fmt.Printf("  [%s]  %s: %s\n", record.Tag(), record.Reason(), record.Duration().String())
			default:
				fmt.Printf("  [%s]  %s\n", record.Tag(), record.Value())
			}
		}
	}
}
```

Output:

```
1_sess
  [Begin]  sess 0 HTTP/1
  [SessOpen]  192.168.50.1 55650 http 192.168.50.10 80 1728889150.256391 26
  [Link]  req 2 rxreq
  [SessClose]  REM_CLOSE: 123ms
  [End]  
2_req
  [Begin]  req 1 rxreq
  [Timestamp]  Start: 21.3ms
  [VCL_use]  boot
  [End]  
```
