// SPDX-License-Identifier: MIT

package html

import (
	"fmt"
	"strings"

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

func curlCommand(tx *vsl.Transaction, cfg PageData) string {
	httpReq, backend, err := processReqBuildForm(tx, cfg)
	if err != nil {
		return err.Error()
	}
	return httpReq.CurlCommand(cfg.ReqBuild.Scheme, backend)
}
