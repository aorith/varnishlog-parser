package render

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/aorith/varnishlog-parser/vsl"
	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/go-echarts/v2/render"
)

type TimestampsForm struct {
	SinceLast   bool
	Timeline    bool
	Events      []string
	OtherEvents bool
}

func (f TimestampsForm) HasEvent(e string) bool {
	for _, event := range f.Events {
		if event == e {
			return true
		}
	}
	return false
}

var TimestampEventLabels = []string{
	"Start",
	"Req",
	"ReqBody",
	"Fetch",
	"Process",
	"Resp",
	"Connected",
	"Bereq",
	"Beresp",
	"BerespBody",
	"Error",
	"Reset",
}

func isKnownEvent(label string) bool {
	for _, event := range TimestampEventLabels {
		if event == label {
			return true
		}
	}
	return false
}

func TimelineChart(txsSet vsl.TransactionSet, f TimestampsForm) templ.Component {
	line := charts.NewLine()
	line.SetGlobalOptions(
		charts.WithTitleOpts(opts.Title{
			Title: "Event Timeline",
		}),
		charts.WithInitializationOpts(opts.Initialization{
			BackgroundColor: "#ffffff",
			Width:           "1000px",
			Height:          "600px",
		}),
		charts.WithXAxisOpts(opts.XAxis{
			Show:  opts.Bool(false),
			Type:  "value",
			Scale: opts.Bool(true),
		}),
		charts.WithYAxisOpts(opts.YAxis{
			Name:         "Duration",
			NameLocation: "center",
			NameGap:      55,
			Type:         "value",
			AxisLabel:    &opts.AxisLabel{Formatter: "{value}s", Rotate: 25},
		}),
		charts.WithLegendOpts(opts.Legend{
			Type:   "scroll",
			Left:   "right",
			Orient: "vertical",
		}),
		charts.WithDataZoomOpts(opts.DataZoom{
			Type:       "slider",
			Start:      0,
			End:        100,
			XAxisIndex: []int{0},
		}),
	)

	visited := make(map[string]bool)
	for i, tx := range txsSet.UniqueRootParents() {
		events := addEvents(tx, f, visited)
		line.AddSeries(
			fmt.Sprintf("Group-%d", i),
			events,
			charts.WithLineChartOpts(opts.LineChart{
				SymbolSize: 5,
			}),
			charts.WithLabelOpts(opts.Label{
				Show:      opts.Bool(true),
				FontSize:  9,
				Formatter: "{@[2]}",
				Color:     "#727272",
			}),
		)
	}

	return renderChartAsTemplComponent(line.RenderSnippet())
}

// Recursively adds the timestamp events in the order they happen
func addEvents(tx *vsl.Transaction, f TimestampsForm, visited map[string]bool) []opts.LineData {
	var events []opts.LineData

	if visited[tx.TXID()] {
		log.Printf("RenderTimestampsTab() -> addEvents: loop detected at transaction %q\n", tx.TXID())
		return nil
	}
	visited[tx.TXID()] = true

	for _, r := range tx.LogRecords() {
		switch record := r.(type) {
		case vsl.TimestampRecord:

			if isKnownEvent(record.EventLabel()) {
				if !f.HasEvent(record.EventLabel()) {
					continue
				}
			} else if !f.OtherEvents {
				continue
			}

			var value float64
			if f.SinceLast {
				value = record.SinceLast().Seconds()
			} else {
				value = record.SinceStart().Seconds()
			}

			events = append(events, opts.LineData{
				Name:  record.EventLabel() + " (" + tx.TXID() + ")",
				Value: []any{record.AbsoluteTime().Nanosecond(), value, record.EventLabel()},
			})

		case vsl.LinkRecord:
			childTx := tx.Children()[record.TXID()]
			if childTx == nil {
				continue
			}

			childEvents := addEvents(childTx, f, visited)
			events = append(events, childEvents...)
		}
	}

	return events
}

func PercentilesLineChart(txsSet vsl.TransactionSet, f TimestampsForm) templ.Component {
	line := charts.NewLine()
	line.SetGlobalOptions(
		charts.WithTitleOpts(opts.Title{
			Title: "Percentiles by Event",
		}),
		charts.WithInitializationOpts(opts.Initialization{
			BackgroundColor: "#ffffff",
			Width:           "1000px",
			Height:          "600px",
		}),
		charts.WithXAxisOpts(opts.XAxis{
			Name: "Percentile",
		}),
		charts.WithYAxisOpts(opts.YAxis{
			Name:         "Duration",
			NameLocation: "center",
			NameGap:      55,
			Type:         "value",
			AxisLabel:    &opts.AxisLabel{Formatter: "{value}s", Rotate: 25},
		}),
		charts.WithLegendOpts(opts.Legend{
			Type:   "scroll",
			Left:   "right",
			Orient: "vertical",
		}),
	)

	percentiles := []float64{50, 75, 90, 99, 99.9, 99.99, 99.999, 99.9999}
	percentileLabels := make([]string, len(percentiles))
	for i, p := range percentiles {
		percentileLabels[i] = strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", p), "0"), ".") + "%"
	}

	line.SetXAxis(percentileLabels)

	events := make(map[string][]time.Duration)
	for _, tx := range txsSet.Transactions() {
		for _, r := range tx.LogRecords() {
			switch record := r.(type) {
			case vsl.TimestampRecord:

				if isKnownEvent(record.EventLabel()) {
					if !f.HasEvent(record.EventLabel()) {
						continue
					}
				} else if !f.OtherEvents {
					continue
				}

				var value time.Duration
				if f.SinceLast {
					value = record.SinceLast()
				} else {
					value = record.SinceStart()
				}
				events[record.EventLabel()] = append(events[record.EventLabel()], value)
			}
		}
	}

	// Sort the event labels so the legend keeps a stable order
	keys := make([]string, 0, len(events))
	for key := range events {
		keys = append(keys, key)
	}
	slices.SortFunc(keys, func(a, b string) int {
		if a > b {
			return -1
		} else if a < b {
			return 1
		}
		return 0
	})

	for _, key := range keys {
		dur := events[key]
		if len(dur) == 0 {
			continue
		}
		s := make([]float64, len(dur))
		for i, v := range dur {
			s[i] = v.Seconds()
		}

		line.AddSeries(
			key,
			generatePercentileLineItems(calcPercentiles(s, percentiles)),
			charts.WithLineChartOpts(opts.LineChart{
				SymbolSize: 5,
			}),
		)
	}

	return renderChartAsTemplComponent(line.RenderSnippet())
}

func generatePercentileLineItems(values []float64) []opts.LineData {
	items := make([]opts.LineData, len(values))
	for i, v := range values {
		items[i] = opts.LineData{Value: v}
	}
	return items
}

func calcPercentiles(s []float64, percentiles []float64) []float64 {
	var r []float64
	for _, p := range percentiles {
		r = append(r, calcPercentile(s, p))
	}
	return r
}

// calcPercentile calculates the p-th percentile (e.g., 50 for median, 75 for 75th percentile)
// from a slice of float64 values.
// reference: https://en.wikipedia.org/wiki/Percentile
func calcPercentile(s []float64, percentile float64) float64 {
	if percentile < 0 || percentile > 100 {
		panic("percentile must be between 0 and 100")
	}

	// Make a sorted copy of the slice
	sortedS := make([]float64, len(s))
	copy(sortedS, s)
	slices.SortFunc(sortedS, func(a, b float64) int {
		if a < b {
			return -1
		} else if a > b {
			return 1
		}
		return 0
	})

	// Calculate the rank position
	n := float64(len(sortedS))
	rank := percentile / 100 * (n - 1)

	// Interpolate between the two closest ranks
	lowerIndex := int(rank)
	upperIndex := lowerIndex + 1
	if upperIndex >= len(sortedS) {
		return sortedS[lowerIndex]
	}

	// Linear interpolation
	weight := rank - float64(lowerIndex)
	return sortedS[lowerIndex] + float64(weight*float64(sortedS[upperIndex]-sortedS[lowerIndex]))
}

// Helper to render the echarts chart as a templ.Component
func renderChartAsTemplComponent(s render.ChartSnippet) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		tmpl := "{{.Element  }} {{.Script}}"
		t := template.New("snippet")
		t, err := t.Parse(tmpl)
		if err != nil {
			return err
		}

		data := struct {
			Element template.HTML
			Script  template.HTML
		}{
			Element: template.HTML(s.Element),
			Script:  template.HTML(s.Script),
		}

		err = t.Execute(w, data)
		if err != nil {
			return err
		}

		return nil
	})
}
