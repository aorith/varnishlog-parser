package summary

import (
	"cmp"
	"slices"

	"github.com/aorith/varnishlog-parser/vsl"
)

// DuplicateHeader is a header name/value pair along with the number of
// times it was seen.
type DuplicateHeader struct {
	Name  string
	Value string
	Count int
}

// DuplicateHeaders returns an slice of headers which appear more than 'minDup' times with
// the same name and value.
func DuplicateHeaders(ts vsl.TransactionSet, txtype vsl.TxType, minDup int) []DuplicateHeader {
	common := make(map[string]*DuplicateHeader)

	for _, tx := range ts.Transactions() {
		if tx.TXType != txtype {
			continue
		}

		for _, r := range tx.ReqHeaders {
			for _, v := range r.Values(true) {
				name := r.Name()
				val := v.Value()

				k, ok := common[name+val]
				if ok {
					k.Count++

					continue
				}

				common[name+val] = &DuplicateHeader{Name: name, Value: val, Count: 1}
			}
		}
	}

	result := []DuplicateHeader{}

	for _, k := range common {
		if k.Count >= minDup {
			result = append(result, *k)
		}
	}

	slices.SortFunc(result, func(a, b DuplicateHeader) int {
		if c := cmp.Compare(b.Count, a.Count); c != 0 {
			return c
		}

		if c := cmp.Compare(a.Name, b.Name); c != 0 {
			return c
		}

		return cmp.Compare(a.Value, b.Value)
	})

	return result
}
