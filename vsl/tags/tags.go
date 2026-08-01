// SPDX-License-Identifier: MIT

// Package tags contains all the known VSL tags
package tags

// Reference: https://varnish-cache.org/docs/6.0/reference/vsl.html

const (
	// Client request header.
	ReqHeader = "ReqHeader"
	// Client response header.
	RespHeader = "RespHeader"
	// Backend request header.
	BereqHeader = "BereqHeader"
	// Backend response header.
	BerespHeader = "BerespHeader"
	// Object header.
	ObjHeader = "ObjHeader"
)

const (
	// Request header unset.
	ReqUnset = "ReqUnset"
	// Response header unset.
	RespUnset = "RespUnset"
	// Backend request unset header.
	BereqUnset = "BereqUnset"
	// Backend response unset header.
	BerespUnset = "BerespUnset"
	// Object header unset.
	ObjUnset = "ObjUnset"
)

const (
	// Marks the start of a VXID.
	Begin = "Begin"
	// Marks the end of a VXID.
	End = "End"
	// ActiveDNS diagnostic message.
	ADNS = "ADNS"
	// Backend selected (reserved, not currently emitted by Varnish).
	Backend = "Backend"
	// Logged when a backend connection is closed.
	BackendClose = "BackendClose"
	// Logged when a new backend connection is opened.
	BackendOpen = "BackendOpen"
	// Backend TLS diagnostic message.
	BackendSSL = "BackendSSL"
	// Logged when a backend connection is started.
	BackendStart = "BackendStart"
	// Logged when a backend connection is reused (keep-alive).
	BackendReuse = "BackendReuse"
	// The result of a backend health probe.
	BackendHealth = "Backend_health"
	// Contains byte counters from backend request processing.
	BereqAcct = "BereqAcct"
	// Backend request method.
	BereqMethod = "BereqMethod"
	// Backend request protocol.
	BereqProtocol = "BereqProtocol"
	// Backend request URL.
	BereqURL = "BereqURL"
	// Backend response protocol.
	BerespProtocol = "BerespProtocol"
	// Backend response reason.
	BerespReason = "BerespReason"
	// Backend response status.
	BerespStatus = "BerespStatus"
	// Response body (XBody response body logging).
	Body = "Body"
	// Bogus HTTP received.
	BogoHeader = "BogoHeader"
	// Brotli - (un)Brotli performed on object.
	Brotli = "Brotli"
	// CLI communication between varnishd master and child process.
	CLI = "CLI"
	// Contains byte counters for CONNECT tunnels.
	ConnectAcct = "ConnectAcct"
	// Crypto diagnostic message (Total Encryption).
	Crypto = "Crypto"
	// DataDome messages, logged by VMOD-DataDome.
	DataDome = "DataDome"
	// Debug messages, can normally be ignored.
	Debug = "Debug"
	// Edgestash diagnostic message.
	Edgestash = "Edgestash"
	// ESI parser error or warning message.
	ESIXMLError = "ESI_xmlerror"
	// Error messages.
	Error = "Error"
	// Object evicted due to ban.
	ExpBan = "ExpBan"
	// Object expiry event.
	ExpKill = "ExpKill"
	// Error while fetching object.
	FetchError = "FetchError"
	// Body fetched from backend.
	FetchBody = "Fetch_Body"
	// Body filters.
	Filters = "Filters"
	// G(un)zip performed on object.
	Gzip = "Gzip"
	// Received HTTP2 frame body.
	H2RxBody = "H2RxBody"
	// Received HTTP2 frame header.
	H2RxHdr = "H2RxHdr"
	// Transmitted HTTP2 frame body.
	H2TxBody = "H2TxBody"
	// Transmitted HTTP2 frame header.
	H2TxHdr = "H2TxHdr"
	// Value added to the object lookup hash.
	Hash = "Hash"
	// Hit object in cache.
	Hit = "Hit"
	// Hit for miss object in cache.
	HitMiss = "HitMiss"
	// Hit for pass object in cache.
	HitPass = "HitPass"
	// Unparsable HTTP request.
	HTTPGarbage = "HttpGarbage"
	// Size of object body.
	Length = "Length"
	// Links to a child VXID.
	Link = "Link"
	// Failed attempt to set HTTP header.
	LostHeader = "LostHeader"
	// MSE4 new object timing data.
	MSE4NewObject = "MSE4_NewObject"
	// MSE4 object payload iteration timing summary.
	MSE4ObjIter = "MSE4_ObjIter"
	// MSE4 persisted chunk memory fault.
	MSE4ChunkFault = "MSE4_ChunkFault"
	// MSE4 cache eviction.
	MSE4Eviction = "MSE4_Eviction"
	// MSE4 YKEY iteration timing and diagnostic message.
	MSE4YKEYIter = "MSE4_YKEY_iter"
	// vmod_nodes debug information.
	Nodes = "Nodes"
	// Informational messages about request handling.
	Notice = "Notice"
	// OCSP information messages.
	OCSP = "OCSP"
	// OCSP error messages.
	OCSPError = "OCSP_Error"
	// Object protocol.
	ObjProtocol = "ObjProtocol"
	// Object response.
	ObjReason = "ObjReason"
	// Object status.
	ObjStatus = "ObjStatus"
	// Pipe byte counts.
	PipeAcct = "PipeAcct"
	// PROXY protocol information.
	Proxy = "Proxy"
	// Unparseble PROXY request.
	ProxyGarbage = "ProxyGarbage"
	// Request handling byte counts.
	ReqAcct = "ReqAcct"
	// Client request method.
	ReqMethod = "ReqMethod"
	// Client request protocol.
	ReqProtocol = "ReqProtocol"
	// Client request start.
	ReqStart = "ReqStart"
	// Request target as received.
	ReqTarget = "ReqTarget"
	// Client request URL.
	ReqURL = "ReqURL"
	// Client response protocol.
	RespProtocol = "RespProtocol"
	// Client response response.
	RespReason = "RespReason"
	// Client response status.
	RespStatus = "RespStatus"
	// Client connection closed.
	SessClose = "SessClose"
	// Client connection accept failed.
	SessError = "SessError"
	// Client connection opened.
	SessOpen = "SessOpen"
	// Where object is stored.
	Storage = "Storage"
	// Client-side TLS diagnostic message.
	TLS = "TLS"
	// TTL set on object.
	TTL = "TTL"
	// Timing information.
	Timestamp = "Timestamp"
	// VCL execution error message.
	VCLError = "VCL_Error"
	// Log statement from VCL.
	VCLLog = "VCL_Log"
	// VCL ACL check results.
	VCLAcl = "VCL_acl"
	// VCL method called.
	VCLCall = "VCL_call"
	// VCL method return value.
	VCLReturn = "VCL_return"
	// VCL trace data.
	VCLTrace = "VCL_trace"
	// VCL in use.
	VCLUse = "VCL_use"
	// VHA6 diagnostic message.
	VHA6 = "VHA6"
	// VSL API warnings and error message.
	VSL = "VSL"
	// Deliver filter accounting.
	VdpAcct = "VdpAcct"
	// Fetch filter accounting.
	VfpAcct = "VfpAcct"
	// WAF diagnostic message.
	WAF = "WAF"
	// Lock order witness records.
	Witness = "Witness"
	// Logs thread start/stop events.
	WorkThread = "WorkThread"
	// XBody vmod diagnostic message.
	XBody = "XBody"
	// YKEY vmod diagnostic message.
	YKEY = "YKEY"
)
