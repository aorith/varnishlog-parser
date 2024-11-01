package cmd

import (
	"log"
	"strings"

	"github.com/aorith/varnishlog-parser/assets"
	"github.com/aorith/varnishlog-parser/pkg/render"
	"github.com/aorith/varnishlog-parser/vsl"
	"github.com/spf13/cobra"
)

// serverCmd represents the server command
var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Test",
	Run: func(cmd *cobra.Command, args []string) {
		test()
	},
}

func init() {
	rootCmd.AddCommand(testCmd)
}

func test() {
	p := vsl.NewTransactionParser(strings.NewReader(assets.VCLComplete1))
	txsSet, err := p.Parse()
	if err != nil {
		log.Println(err)
		return
	}

	for _, tx := range txsSet.UniqueRootParents() {
		log.Println("\n", render.SequenceDiagram(tx))
		log.Println("")
	}

	// txs := txsSet.Transactions()

	// // Iterate all tx
	// for _, tx := range txs {
	// 	log.Printf("%v\n", tx.TXID())
	// 	for _, r := range tx.LogRecords() {
	// 		switch l := r.(type) {
	// 		case vsl.HeaderRecord:
	// 			log.Printf("   Header: %q Val: %q\n", l.Header(), l.HeaderValue())
	// 		case vsl.BaseRecord:
	// 			log.Printf("   %v\n", l.Tag())
	// 		default:
	// 			log.Println("NOSE")
	// 		}
	// 	}
	// }
	//
	// // Example: iterate all without type switching to find specific tags
	// for _, tx := range txs {
	// 	log.Printf("Raw: %v\n", tx.RawLog())
	// 	log.Printf("TXID: %v\n", tx.TXID())
	// 	log.Printf("VXID: %v\n", tx.VXID())
	// 	log.Printf("Level: %v\n", tx.Level())
	// 	log.Printf("Type: %v\n", tx.Type())
	// 	log.Println("")
	// 	for _, r := range tx.LogRecords() {
	// 		if r.Tag() == tag.Link {
	// 			record := r.(vsl.LinkRecord)
	// 			log.Println("HELLO", record.Type(), record.VXID())
	// 		}
	// 	}
	// }
	//
	// txsGroups := txsSet.GroupRelatedTransactions()
	// for i, g := range txsGroups {
	// 	log.Printf("Group %d: %d\n", i, len(g))
	// }
}
