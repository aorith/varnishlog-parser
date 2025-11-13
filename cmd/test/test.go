package main

import (
	"fmt"
	"strings"

	"github.com/aorith/varnishlog-parser/assets"
	"github.com/aorith/varnishlog-parser/vsl"
	"github.com/aorith/varnishlog-parser/vsl/summary"
)

func main() {
	ts, err := vsl.NewTransactionParser(strings.NewReader(assets.VCLComplete1)).Parse()
	if err != nil {
		panic(err)
	}

	// t := ts.Transactions()[1]
	// j, err := json.Marshal(t)
	// if err != nil {
	// 	panic(err)
	// }
	// fmt.Println(string(j))

	// j2, err := json.Marshal(summary.Summary(ts))
	// if err != nil {
	// 	panic(err)
	// }
	// fmt.Println(string(j2))

	events := summary.TimestampEventsSummary(ts)
	for n, e := range events {
		fmt.Println(n, e)
	}
}
