// Code generated by templ - DO NOT EDIT.

// templ: version: v0.2.793
package content

//lint:file-ignore SA4006 This context is only used if a nested component is present.

import "github.com/a-h/templ"
import templruntime "github.com/a-h/templ/runtime"

import (
	"fmt"
	"strings"

	"github.com/aorith/varnishlog-parser/vsl"
	"github.com/aorith/varnishlog-parser/vsl/header"
)

const (
	SendToDomain = iota
	SendToBackend
	SendToLocalhost
	SendToCustom
)

type ReqBuilderForm struct {
	TXID            string
	HTTPS           bool
	OriginalHeaders bool
	OriginalURL     bool
	ResolveTo       int
	CustomResolve   string
}

func ReqBuilderTab(txsSet vsl.TransactionSet) templ.Component {
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
		_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString("<div id=\"tabRequest\" class=\"tabcontent\"><p>Here you can generate a <a href=\"https://curl.se/\" target=\"_blank\">cURL</a> command based on parsed transactions VSL tags.</p><form id=\"headerForm\" hx-post=\"/reqbuilder/\" hx-target=\"#reqBuilderResults\" hx-swap=\"innerHTML settle:0.3s\" hx-include=\"[name=&#39;logs&#39;]\"><fieldset><legend>Transaction: </legend> <select id=\"transactionSelect\" name=\"transaction\">")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		for _, tx := range txsSet.Transactions() {
			if tx.Type() != vsl.TxTypeSession {
				_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString("<option value=\"")
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
				var templ_7745c5c3_Var2 string
				templ_7745c5c3_Var2, templ_7745c5c3_Err = templ.JoinStringErrs(tx.TXID())
				if templ_7745c5c3_Err != nil {
					return templ.Error{Err: templ_7745c5c3_Err, FileName: `pkg/server/templates/content/tab_reqbuilder.templ`, Line: 42, Col: 32}
				}
				_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var2))
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
				_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString("\">")
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
				var templ_7745c5c3_Var3 string
				templ_7745c5c3_Var3, templ_7745c5c3_Err = templ.JoinStringErrs(tx.TXID())
				if templ_7745c5c3_Err != nil {
					return templ.Error{Err: templ_7745c5c3_Err, FileName: `pkg/server/templates/content/tab_reqbuilder.templ`, Line: 42, Col: 46}
				}
				_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var3))
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
				_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString("</option>")
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
			}
		}
		_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString("</select></fieldset><br><fieldset><legend>Protocol: </legend> <label><input type=\"checkbox\" name=\"https\" checked> https</label></fieldset><fieldset><legend>URL: </legend> <label><input type=\"radio\" name=\"urlType\" value=\"original\" checked> Original</label> <label><input type=\"radio\" name=\"urlType\" value=\"final\"> Final</label></fieldset><fieldset><legend>Headers: </legend> <label><input type=\"radio\" name=\"headerType\" value=\"original\" checked> Original</label> <label><input type=\"radio\" name=\"headerType\" value=\"final\"> Final</label></fieldset><br><fieldset><legend>Send To: </legend> <label><input type=\"radio\" name=\"sendTo\" value=\"domain\" checked> Domain</label> <label><input type=\"radio\" name=\"sendTo\" value=\"localhost\"> Localhost</label><br><br><label><input type=\"radio\" name=\"sendTo\" value=\"backend\"> Backend:<br><select id=\"transactionBackend\" name=\"transactionBackend\">")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		for _, tx := range txsSet.Transactions() {
			if tx.Type() == vsl.TxTypeBereq {
				_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString("<option value=\"")
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
				var templ_7745c5c3_Var4 string
				templ_7745c5c3_Var4, templ_7745c5c3_Err = templ.JoinStringErrs(getBackend(tx))
				if templ_7745c5c3_Err != nil {
					return templ.Error{Err: templ_7745c5c3_Err, FileName: `pkg/server/templates/content/tab_reqbuilder.templ`, Line: 89, Col: 38}
				}
				_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var4))
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
				_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString("\">")
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
				var templ_7745c5c3_Var5 string
				templ_7745c5c3_Var5, templ_7745c5c3_Err = templ.JoinStringErrs(tx.TXID())
				if templ_7745c5c3_Err != nil {
					return templ.Error{Err: templ_7745c5c3_Err, FileName: `pkg/server/templates/content/tab_reqbuilder.templ`, Line: 89, Col: 52}
				}
				_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var5))
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
				_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(" (")
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
				var templ_7745c5c3_Var6 string
				templ_7745c5c3_Var6, templ_7745c5c3_Err = templ.JoinStringErrs(getBackend(tx))
				if templ_7745c5c3_Err != nil {
					return templ.Error{Err: templ_7745c5c3_Err, FileName: `pkg/server/templates/content/tab_reqbuilder.templ`, Line: 89, Col: 72}
				}
				_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var6))
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
				_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(")</option>")
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
			}
		}
		_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString("</select></label><br><br><label><input type=\"radio\" name=\"sendTo\" value=\"custom\"> Custom: <input type=\"text\" name=\"customResolve\" pattern=\".*:.*\" placeholder=\"&lt;IP&gt;:&lt;PORT&gt;\"></label></fieldset><br><button class=\"btn loading\">Generate</button></form><br><div id=\"reqBuilderResults\"></div></div>")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		return templ_7745c5c3_Err
	})
}

func ReqBuild(txsSet vsl.TransactionSet, tx *vsl.Transaction, f ReqBuilderForm) templ.Component {
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
		templ_7745c5c3_Var7 := templ.GetChildren(ctx)
		if templ_7745c5c3_Var7 == nil {
			templ_7745c5c3_Var7 = templ.NopComponent
		}
		ctx = templ.ClearChildren(ctx)
		_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString("<pre class=\"fade-me-in\"><code>")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		templ_7745c5c3_Err = templ.Raw(curlHeaders(tx, f)).Render(ctx, templ_7745c5c3_Buffer)
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString("</code></pre>")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		return templ_7745c5c3_Err
	})
}

func curlHeaders(t *vsl.Transaction, f ReqBuilderForm) string {
	var s strings.Builder

	hdrState := header.NewHeaderState(t.LogRecords(), false)

	// Find the host header
	hostHdr := hdrState.FindHeader("host", f.OriginalHeaders, true)
	if hostHdr == nil {
		return "Host header not found!"
	}

	// Find the URL
	var url vsl.Record
	if f.OriginalURL {
		url = t.FirstRecordOfType(vsl.URLRecord{})
	} else {
		url = t.LastRecordOfType(vsl.URLRecord{})
	}
	if url == nil {
		return "URL not found!"
	}

	port := "80"
	if f.HTTPS {
		port = "443"
		s.WriteString("curl https://" + hostHdr.HeaderValue() + url.Value() + " \\\n")
	} else {
		s.WriteString("curl http://" + hostHdr.HeaderValue() + url.Value() + " \\\n")
	}

	// Add headers
	var hdrVal string
	for _, hc := range hdrState {
		if hc.Header() == "host" || (f.OriginalHeaders && !hc.IsOriginalHeader()) {
			continue
		}
		if f.OriginalHeaders {
			hdrVal = hc.OriginalValue()
		} else {
			hdrVal = hc.FinalValue()
		}
		s.WriteString(fmt.Sprintf(`    -H "%s: %s" \`+"\n", hc.Header(), hdrVal))
	}

	// Fixed options
	s.WriteString("    -s -v -o /dev/null")

	// Optional resolve
	switch f.ResolveTo {
	case SendToLocalhost:
		s.WriteString(" \\\n    --resolve " + hostHdr.HeaderValue() + ":" + port + ":127.0.0.1")
	case SendToBackend, SendToCustom:
		custom := strings.SplitN(f.CustomResolve, ":", 2)
		if custom[0] == "none" {
			return "Backed address not found for selected transaction."
		}
		if len(custom) < 2 {
			return "Incorrect backend address."
		}
		s.WriteString(" \\\n    --resolve " + hostHdr.HeaderValue() + ":" + custom[1] + ":" + custom[0])

		if (f.HTTPS && custom[1] == "80") || (!f.HTTPS && custom[1] == "443") {
			s.WriteString("\n\n# Incorrect protocol selected?")
		}
	}

	return s.String()
}

func getBackend(t *vsl.Transaction) string {
	r := t.FirstRecordOfType(vsl.BackendOpenRecord{})
	if r == nil {
		return "none"
	}
	record := r.(vsl.BackendOpenRecord)
	return fmt.Sprintf("%s:%d", record.RemoteAddr().String(), record.RemotePort())
}

var _ = templruntime.GeneratedTemplate
