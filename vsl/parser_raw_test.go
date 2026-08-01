// SPDX-License-Identifier: MIT

package vsl_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/aorith/varnishlog-parser/assets"
	"github.com/aorith/varnishlog-parser/vsl"
)

// rawFixturePairs are captures of the same hurl scenario, once in grouped
// ("-g request"/"-g session") format and once in flat ("-g raw") format.
// The two captures come from separate docker-test runs, so concrete values
// (dates, container hostnames, client/backend versions) differ - only the
// transaction structure they produce should match.
var rawFixturePairs = []struct {
	name    string
	grouped string
	raw     string
}{
	{"backend-retry", assets.VCLBackendRetry, assets.VCLBackendRetryRaw},
	{"esi-1", assets.VCLESI1, assets.VCLESI1Raw},
	{"esi-synth", assets.VCLESISynth, assets.VCLESISynthRaw},
	{"req-restart", assets.VCLRestart, assets.VCLRestartRaw},
	{"simple-post", assets.VCLSimplePOST, assets.VCLSimplePOSTRaw},
	{"streaming-hit", assets.VCLStreamingHit, assets.VCLStreamingHitRaw},
}

func mustParse(t *testing.T, name, rawLog string) vsl.TransactionSet {
	t.Helper()

	p := vsl.NewTransactionParser(strings.NewReader(rawLog))

	ts, err := p.Parse()
	if err != nil {
		t.Fatalf("%s: Parse() failed: %s", name, err)
	}

	return ts
}

// assertTransactionSubset checks that every transaction in subset has a
// structurally matching (TXType/Parent/Reason/ESILevel/Children) counterpart
// in superset, by VXID.
//
// This should be fed of txs generated using the same request and VCL but with
// different grouping options (raw, request, session, vxid) and with or without
// verbose mode (-v) with varnishlog.
func assertTransactionSubset(t *testing.T, subsetName string, subset vsl.TransactionSet, supersetName string, superset vsl.TransactionSet) {
	t.Helper()

	for _, stx := range subset.Transactions() {
		sup := superset.GetTX(stx.VXID)
		if sup == nil {
			t.Fatalf("vxid %d: present in %s but not in %s", stx.VXID, subsetName, supersetName)
		}

		if stx.TXType != sup.TXType {
			t.Errorf("vxid %d: TXType %s=%v %s=%v", stx.VXID, subsetName, stx.TXType, supersetName, sup.TXType)
		}

		if stx.Parent != sup.Parent {
			t.Errorf("vxid %d: Parent %s=%v %s=%v", stx.VXID, subsetName, stx.Parent, supersetName, sup.Parent)
		}

		if stx.Reason != sup.Reason {
			t.Errorf("vxid %d: Reason %s=%q %s=%q", stx.VXID, subsetName, stx.Reason, supersetName, sup.Reason)
		}

		if stx.ESILevel != sup.ESILevel {
			t.Errorf("vxid %d: ESILevel %s=%d %s=%d", stx.VXID, subsetName, stx.ESILevel, supersetName, sup.ESILevel)
		}

		sc := slices.Clone(stx.Children)
		pc := slices.Clone(sup.Children)

		slices.Sort(sc)
		slices.Sort(pc)

		if !slices.Equal(sc, pc) {
			t.Errorf("vxid %d: Children %s=%v %s=%v", stx.VXID, subsetName, sc, supersetName, pc)
		}
	}
}

// assertEquivalentTransactionGraphs checks that a and b parse to exactly the
// same transaction graph: same VXID set, and each one structurally matching.
func assertEquivalentTransactionGraphs(t *testing.T, aName string, a vsl.TransactionSet, bName string, b vsl.TransactionSet) {
	t.Helper()

	if la, lb := len(a.Transactions()), len(b.Transactions()); la != lb {
		t.Fatalf("transaction count mismatch: %s=%d %s=%d", aName, la, bName, lb)
	}

	assertTransactionSubset(t, aName, a, bName, b)
}

// TestRawFormatEquivalence parses the same hurl scenario captured in both
// grouped and "-g raw" format and checks they produce equivalent
// transaction graphs.
func TestRawFormatEquivalence(t *testing.T) {
	for _, tt := range rawFixturePairs {
		t.Run(tt.name, func(t *testing.T) {
			grouped := mustParse(t, tt.name+"-grouped", tt.grouped)
			raw := mustParse(t, tt.name+"-raw", tt.raw)

			assertEquivalentTransactionGraphs(t, "grouped", grouped, "raw", raw)
		})
	}
}

// TestVerboseFormatEquivalence parses the req-restart scenario captured with
// "-v" (verbose) across all four grouping modes and checks each transaction
// it produces has a structurally matching counterpart in the plain
// (non-verbose) "-g session" grouped capture.
func TestVerboseFormatEquivalence(t *testing.T) {
	baseline := mustParse(t, "req-restart", assets.VCLRestart)

	verboseFixtures := []struct {
		name string
		log  string
	}{
		{"verbose-g-session", assets.VCLRestartVerboseSession},
		{"verbose-g-request", assets.VCLRestartVerboseRequest},
		{"verbose-g-vxid", assets.VCLRestartVerboseVXID},
		{"verbose-g-raw", assets.VCLRestartVerboseRaw},
	}

	for _, tt := range verboseFixtures {
		t.Run(tt.name, func(t *testing.T) {
			verbose := mustParse(t, tt.name, tt.log)

			assertTransactionSubset(t, tt.name, verbose, "grouped", baseline)
		})
	}
}

// TestRawFormatIgnoresVXIDZero checks that non-transactional "-g raw" lines
// (VXID 0, e.g. CLI ping/pong) don't produce transactions or errors.
func TestRawFormatIgnoresVXIDZero(t *testing.T) {
	for _, tt := range rawFixturePairs {
		if !strings.Contains(tt.raw, "\n0 CLI") && !strings.Contains(tt.raw, " 0 CLI") {
			continue
		}

		t.Run(tt.name, func(t *testing.T) {
			ts := mustParse(t, tt.name, tt.raw)

			if got := ts.GetTX(0); got != nil {
				t.Errorf("expected no transaction for vxid 0, got: %+v", got)
			}
		})
	}
}

// TestRawFormatInterleaved proves the accumulator handles records from
// concurrently-open transactions interleaved line by line, not just the
// contiguous-per-vxid case the sample fixtures happen to show.
func TestRawFormatInterleaved(t *testing.T) {
	const rawLog = `1 Begin          c sess 0 HTTP/1
1 Link           c req 2 rxreq
2 Begin          c req 1 rxreq
1 SessOpen       c 192.168.50.1 1 http 192.168.50.10 80 1785569916.1 1
2 ReqMethod      c GET
2 Link           c bereq 3 fetch
3 Begin          b bereq 2 fetch
2 ReqURL         c /interleaved
3 BereqMethod    b GET
3 End            b
2 End            c
1 SessClose      c REM_CLOSE 0.001
1 End            c
`

	ts := mustParse(t, "interleaved", rawLog)

	if got := len(ts.Transactions()); got != 3 {
		t.Fatalf("expected 3 transactions, got %d", got)
	}

	sess := ts.GetTX(1)
	req := ts.GetTX(2)
	bereq := ts.GetTX(3)

	if sess == nil || req == nil || bereq == nil {
		t.Fatalf("missing transaction(s): sess=%v req=%v bereq=%v", sess, req, bereq)
	}

	if req.RecordByTag("ReqURL", true) == nil {
		t.Error("vxid 2: expected a ReqURL record despite interleaving")
	}

	if req.RecordByTag("ReqMethod", true) == nil {
		t.Error("vxid 2: expected a ReqMethod record despite interleaving")
	}

	if bereq.RecordByTag("BereqMethod", true) == nil {
		t.Error("vxid 3: expected a BereqMethod record despite interleaving")
	}

	if !slices.Contains(sess.Children, 2) {
		t.Errorf("vxid 1: expected child 2, got children %v", sess.Children)
	}

	if !slices.Contains(req.Children, 3) {
		t.Errorf("vxid 2: expected child 3, got children %v", req.Children)
	}
}
