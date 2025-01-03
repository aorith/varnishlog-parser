// Code generated by templ - DO NOT EDIT.

// templ: version: v0.3.819
package content

//lint:file-ignore SA4006 This context is only used if a nested component is present.

import "github.com/a-h/templ"
import templruntime "github.com/a-h/templ/runtime"

import (
	"context"
	"io"
	"log"

	"github.com/aorith/varnishlog-parser/vsl"
	"github.com/aorith/varnishlog-parser/vsl/header"
)

func HeadersTab(txsSet vsl.TransactionSet) templ.Component {
	visited := make(map[string]bool)
	return headersTab(txsSet, visited)
}

func headersTab(txsSet vsl.TransactionSet, visited map[string]bool) templ.Component {
	return templruntime.GeneratedTemplate(func(templ_7745c5c3_Input templruntime.GeneratedComponentInput) (templ_7745c5c3_Err error) {
		templ_7745c5c3_W, ctx := templ_7745c5c3_Input.Writer, templ_7745c5c3_Input.Context
		if templ_7745c5c3_CtxErr := ctx.Err(); templ_7745c5c3_CtxErr != nil {
			return templ_7745c5c3_CtxErr
		}
		templ_7745c5c3_Buffer, templ_7745c5c3_IsBuffer := templruntime.GetBuffer(templ_7745c5c3_W)
		if !templ_7745c5c3_IsBuffer {
			defer func() {
				templ_7745c5c3_BufErr := templruntime.ReleaseBuffer(templ_7745c5c3_Buffer)
				if templ_7745c5c3_Err == nil {
					templ_7745c5c3_Err = templ_7745c5c3_BufErr
				}
			}()
		}
		ctx = templ.InitializeContext(ctx)
		templ_7745c5c3_Var1 := templ.GetChildren(ctx)
		if templ_7745c5c3_Var1 == nil {
			templ_7745c5c3_Var1 = templ.NopComponent
		}
		ctx = templ.ClearChildren(ctx)
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 1, "<div id=\"tabHeaders\" class=\"tabcontent\"><p>This tab displays the state of HTTP headers, organized into transaction groups. For each transaction of type \"request\" or \"backend request,\" four tables show the state of the headers.</p><p>Headers have four states:</p><table class=\"headers-legend\"><tr><th class=\"diffOriginal\">Original:</th><td>Headers present before any VCL processing, as initially sent by the client.</td></tr><tr><th class=\"diffModified\">Modified:</th><td>Headers that originated from the client but were subsequently modified in VCL.</td></tr><tr><th class=\"diffDeleted\">Deleted:</th><td>Original headers that have been removed during VCL processing.</td></tr><tr><th class=\"diffAdded\">Added:</th><td>New headers introduced by VCL that were not part of the original request.</td></tr></table><br><br>")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		for _, root := range txsSet.UniqueRootParents() {
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 2, "<h4>Headers for tx group \"")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			var templ_7745c5c3_Var2 string
			templ_7745c5c3_Var2, templ_7745c5c3_Err = templ.JoinStringErrs(root.TXID())
			if templ_7745c5c3_Err != nil {
				return templ.Error{Err: templ_7745c5c3_Err, FileName: `pkg/server/templates/content/tab_headers.templ`, Line: 45, Col: 42}
			}
			_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var2))
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 3, "\"</h4><div class=\"headers\">")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			templ_7745c5c3_Err = renderHeaderTree(root, visited).Render(ctx, templ_7745c5c3_Buffer)
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 4, "</div>")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
		}
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 5, "</div>")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		return nil
	})
}

func renderHeaderTree(tx *vsl.Transaction, visited map[string]bool) templ.Component {
	if visited[tx.TXID()] {
		log.Printf("renderHeaderTree(): loop detected at transaction %q\n", tx.TXID())
		return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
			return nil
		})
	}
	visited[tx.TXID()] = true

	reqHeadersState := header.NewHeaderState(tx.LogRecords(), false)
	respHeadersState := header.NewHeaderState(tx.LogRecords(), true)
	return renderTxHeaderTree(tx, reqHeadersState, respHeadersState, visited)
}

func renderTxHeaderTree(tx *vsl.Transaction, reqHeadersState, respHeadersState header.HeaderStates, visited map[string]bool) templ.Component {
	return templruntime.GeneratedTemplate(func(templ_7745c5c3_Input templruntime.GeneratedComponentInput) (templ_7745c5c3_Err error) {
		templ_7745c5c3_W, ctx := templ_7745c5c3_Input.Writer, templ_7745c5c3_Input.Context
		if templ_7745c5c3_CtxErr := ctx.Err(); templ_7745c5c3_CtxErr != nil {
			return templ_7745c5c3_CtxErr
		}
		templ_7745c5c3_Buffer, templ_7745c5c3_IsBuffer := templruntime.GetBuffer(templ_7745c5c3_W)
		if !templ_7745c5c3_IsBuffer {
			defer func() {
				templ_7745c5c3_BufErr := templruntime.ReleaseBuffer(templ_7745c5c3_Buffer)
				if templ_7745c5c3_Err == nil {
					templ_7745c5c3_Err = templ_7745c5c3_BufErr
				}
			}()
		}
		ctx = templ.InitializeContext(ctx)
		templ_7745c5c3_Var3 := templ.GetChildren(ctx)
		if templ_7745c5c3_Var3 == nil {
			templ_7745c5c3_Var3 = templ.NopComponent
		}
		ctx = templ.ClearChildren(ctx)
		if tx.Type() != vsl.TxTypeSession {
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 6, "<details><summary>")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			var templ_7745c5c3_Var4 string
			templ_7745c5c3_Var4, templ_7745c5c3_Err = templ.JoinStringErrs(tx.TXID())
			if templ_7745c5c3_Err != nil {
				return templ.Error{Err: templ_7745c5c3_Err, FileName: `pkg/server/templates/content/tab_headers.templ`, Line: 70, Col: 23}
			}
			_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var4))
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 7, "</summary><table class=\"headers-table\"><tr><tr class=\"hdr-type\"><th colspan=\"2\"><abbr title=\"Headers as initially sent by the client, before any VCL processing.\">Original Headers</abbr></th></tr>")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			for _, hc := range reqHeadersState {
				if hc.IsOriginalHeader() {
					templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 8, "<tr>")
					if templ_7745c5c3_Err != nil {
						return templ_7745c5c3_Err
					}
					templ_7745c5c3_Err = renderHeaderDiff(hc, true, getHeaderDiffAttrs(header.OriginalHdr)).Render(ctx, templ_7745c5c3_Buffer)
					if templ_7745c5c3_Err != nil {
						return templ_7745c5c3_Err
					}
					templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 9, "</tr>")
					if templ_7745c5c3_Err != nil {
						return templ_7745c5c3_Err
					}
				}
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 10, "<tr class=\"hdr-type\"><th colspan=\"2\"><abbr title=\"State of the headers after VCL processing.\">VCL Headers</abbr></th></tr>")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			for _, hc := range reqHeadersState {
				templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 11, "<tr>")
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
				templ_7745c5c3_Err = renderHeaderDiff(hc, false, getHeaderDiffAttrs(hc.State())).Render(ctx, templ_7745c5c3_Buffer)
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
				templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 12, "</tr>")
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 13, "<tr class=\"hdr-type\"><th colspan=\"2\"><abbr title=\"Headers as initially sent by the client, before any VCL processing.\">Original Response Headers</abbr></th></tr>")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			for _, hc := range respHeadersState {
				if hc.IsOriginalHeader() {
					templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 14, "<tr>")
					if templ_7745c5c3_Err != nil {
						return templ_7745c5c3_Err
					}
					templ_7745c5c3_Err = renderHeaderDiff(hc, true, getHeaderDiffAttrs(header.OriginalHdr)).Render(ctx, templ_7745c5c3_Buffer)
					if templ_7745c5c3_Err != nil {
						return templ_7745c5c3_Err
					}
					templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 15, "</tr>")
					if templ_7745c5c3_Err != nil {
						return templ_7745c5c3_Err
					}
				}
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 16, "<tr class=\"hdr-type\"><th colspan=\"2\"><abbr title=\"State of the response headers after VCL processing.\">VCL Response Headers</abbr></th></tr>")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			for _, hc := range respHeadersState {
				templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 17, "<tr>")
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
				templ_7745c5c3_Err = renderHeaderDiff(hc, false, getHeaderDiffAttrs(hc.State())).Render(ctx, templ_7745c5c3_Buffer)
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
				templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 18, "</tr>")
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 19, "</tr></table></details> ")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
		}
		if len(tx.ChildrenSortedByVXID()) > 0 {
			for _, c := range tx.ChildrenSortedByVXID() {
				templ_7745c5c3_Err = renderHeaderTree(c, visited).Render(ctx, templ_7745c5c3_Buffer)
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
			}
		}
		return nil
	})
}

func getHeaderDiffAttrs(state int) templ.Attributes {
	switch state {
	case header.OriginalHdr:
		return templ.Attributes{"class": "diffOriginal"}
	case header.AddedHdr:
		return templ.Attributes{"class": "diffAdded"}
	case header.ModifiedHdr:
		return templ.Attributes{"class": "diffModified"}
	case header.DeletedHdr:
		return templ.Attributes{"class": "diffDeleted"}
	}
	return templ.Attributes{"class": "diffOriginal"}
}

func renderHeaderDiff(hc header.HeaderState, original bool, attrs templ.Attributes) templ.Component {
	return templruntime.GeneratedTemplate(func(templ_7745c5c3_Input templruntime.GeneratedComponentInput) (templ_7745c5c3_Err error) {
		templ_7745c5c3_W, ctx := templ_7745c5c3_Input.Writer, templ_7745c5c3_Input.Context
		if templ_7745c5c3_CtxErr := ctx.Err(); templ_7745c5c3_CtxErr != nil {
			return templ_7745c5c3_CtxErr
		}
		templ_7745c5c3_Buffer, templ_7745c5c3_IsBuffer := templruntime.GetBuffer(templ_7745c5c3_W)
		if !templ_7745c5c3_IsBuffer {
			defer func() {
				templ_7745c5c3_BufErr := templruntime.ReleaseBuffer(templ_7745c5c3_Buffer)
				if templ_7745c5c3_Err == nil {
					templ_7745c5c3_Err = templ_7745c5c3_BufErr
				}
			}()
		}
		ctx = templ.InitializeContext(ctx)
		templ_7745c5c3_Var5 := templ.GetChildren(ctx)
		if templ_7745c5c3_Var5 == nil {
			templ_7745c5c3_Var5 = templ.NopComponent
		}
		ctx = templ.ClearChildren(ctx)
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 20, "<th>")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		var templ_7745c5c3_Var6 string
		templ_7745c5c3_Var6, templ_7745c5c3_Err = templ.JoinStringErrs(hc.Header())
		if templ_7745c5c3_Err != nil {
			return templ.Error{Err: templ_7745c5c3_Err, FileName: `pkg/server/templates/content/tab_headers.templ`, Line: 127, Col: 18}
		}
		_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var6))
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 21, "</th><td")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		templ_7745c5c3_Err = templ.RenderAttributes(ctx, templ_7745c5c3_Buffer, attrs)
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 22, ">")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		if original {
			var templ_7745c5c3_Var7 string
			templ_7745c5c3_Var7, templ_7745c5c3_Err = templ.JoinStringErrs(hc.OriginalValue())
			if templ_7745c5c3_Err != nil {
				return templ.Error{Err: templ_7745c5c3_Err, FileName: `pkg/server/templates/content/tab_headers.templ`, Line: 130, Col: 23}
			}
			_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var7))
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
		} else {
			var templ_7745c5c3_Var8 string
			templ_7745c5c3_Var8, templ_7745c5c3_Err = templ.JoinStringErrs(hc.FinalValue())
			if templ_7745c5c3_Err != nil {
				return templ.Error{Err: templ_7745c5c3_Err, FileName: `pkg/server/templates/content/tab_headers.templ`, Line: 132, Col: 20}
			}
			_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var8))
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
		}
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 23, "</td>")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		return nil
	})
}

var _ = templruntime.GeneratedTemplate
