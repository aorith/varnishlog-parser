// SPDX-License-Identifier: MIT

package tags

// Descriptions gives a short explanation for a curated subset of VSL tags, used to
// render a tooltip in the VCL Log Tree view. Not every tag is listed here: most are
// self-explanatory from their name and value alone.
var Descriptions = map[string]string{ // nolint:gosec
	VCLCall:      "The VCL subroutine Varnish is about to execute, e.g. vcl_recv or vcl_backend_fetch.",
	VCLReturn:    "The action returned by the VCL subroutine, deciding what happens next, e.g. hash, deliver, pipe.",
	VCLAcl:       "Result of an ACL (access control list) match performed in VCL.",
	Link:         "Links this transaction to a child transaction spawned from it, such as a backend fetch or an ESI include.",
	Hit:          "The request was served from cache; includes the cached object's VXID, TTL, grace and keep values.",
	HitMiss:      "Hit-for-miss: an object marked uncacheable was found, so the request passes to the backend instead of being served from cache.",
	HitPass:      "Hit-for-pass: a previous response told Varnish not to cache this request, so it passes straight to the backend.",
	Gzip:         "Details of a gzip/gunzip transformation performed on the object body.",
	FetchBody:    "How the response body was fetched and stored, e.g. length, chunked or until EOF.",
	ESIXMLError:  "An error or warning from the ESI (Edge Side Includes) parser.",
	TTL:          "TTL, grace and keep values assigned to the object, and the source that set them (RFC, VCL, etc.).",
	Storage:      "The storage backend (stevedore) selected to store the object.",
	BogoHeader:   "A malformed HTTP header line that could not be parsed.",
	ReqAcct:      "Byte counts for the client request/response: header, body and total bytes sent to (Tx) and received from (Rx) the client.",
	BereqAcct:    "Byte counts for the backend request/response: header, body and total bytes sent to (Tx) and received from (Rx) the backend.",
	PipeAcct:     "Byte counts for a piped connection: request headers, plus bytes piped in each direction between client and backend.",
	VfpAcct:      "Byte count processed by a fetch filter (VFP) applied to the response body.",
	VdpAcct:      "Byte count processed by a deliver filter (VDP) applied to the response body.",
	BackendReuse: "An existing (keep-alive) backend connection was reused instead of opening a new one.",
	YKEY:         "Diagnostic message from the ykey vmod: namespace or key, the action performed (ADD, PURGE or STAT), and the key name or blob length.",
}
