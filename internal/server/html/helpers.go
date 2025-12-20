// SPDX-License-Identifier: MIT

package html

import (
	"bytes"
	"fmt"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"

	"github.com/aorith/varnishlog-parser/render"
	"github.com/aorith/varnishlog-parser/vsl"
)

func processReqBuildForm(tx *vsl.Transaction, cfg PageData) (*render.HTTPRequest, *render.Backend, error) {
	var backend *render.Backend

	c := cfg.ReqBuild
	switch c.ConnectTo {
	case "backend":
		switch c.Backend {
		case "none":
			backend = nil

		case "auto":
			if tx.TXType == vsl.TxTypeBereq {
				host, port, err := render.ParseBackend(tx.GetBackendConnStr())
				if err != nil {
					return nil, nil, fmt.Errorf("parsing backend failed: %q", err)
				}
				backend = render.NewBackend(host, port)
			}

		default:
			host, port, err := render.ParseBackend(c.Backend)
			if err != nil {
				return nil, nil, fmt.Errorf("failure parsing backend string: %q", err)
			}
			backend = render.NewBackend(host, port)
		}

	case "custom":
		if c.ConnectCustom == "" {
			return nil, nil, fmt.Errorf("parameter ConnectCustom is empty")
		}
		host, port, err := render.ParseBackend(c.ConnectCustom)
		if err != nil {
			return nil, nil, fmt.Errorf("error parsing backend: %q", err)
		}
		backend = render.NewBackend(host, port)

	default:
		return nil, nil, fmt.Errorf("ConnectTo has an incorrect value: %s", c.ConnectTo)
	}

	excludedHeaders := strings.Split(c.ExcludedHeaders, ",")
	for i, n := range excludedHeaders {
		excludedHeaders[i] = strings.TrimSpace(n)
	}

	httpReq, err := render.NewHTTPRequest(tx, c.ReceivedHeaders, excludedHeaders)
	if err != nil {
		return nil, nil, fmt.Errorf("failure creating HTTPRequest object: %q", err)
	}

	return httpReq, backend, nil
}

func applyChromaStyle(text, lang string) string {
	fallback := "<pre><code>" + text + "</code></pre>"

	lexer := lexers.Get(lang)
	formatter := chromahtml.New(chromahtml.WithClasses(true), chromahtml.WithCSSComments(false), chromahtml.ClassPrefix("chr_"))
	iterator, err := lexer.Tokenise(nil, text)
	if err != nil {
		return fallback
	}

	s := bytes.NewBuffer(nil)
	// Style is done via classes at cmd/chroma
	err = formatter.Format(s, styles.Fallback, iterator)
	if err != nil {
		return fallback
	}

	return s.String()
}

func curlCommand(tx *vsl.Transaction, cfg PageData) string {
	httpReq, backend, err := processReqBuildForm(tx, cfg)
	if err != nil {
		return fmt.Sprintf(`<pre>%s</pre>`, err.Error())
	}
	return applyChromaStyle(httpReq.CurlCommand(cfg.ReqBuild.Scheme, backend), "bash")
}

func hurlFile(tx *vsl.Transaction, cfg PageData) string {
	httpReq, backend, err := processReqBuildForm(tx, cfg)
	if err != nil {
		return fmt.Sprintf(`<pre>%s</pre>`, err.Error())
	}
	return applyChromaStyle(httpReq.HurlFile(cfg.ReqBuild.Scheme, backend), "properties")
}
