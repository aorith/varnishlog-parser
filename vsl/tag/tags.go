package tag

// Reference: https://varnish-cache.org/docs/6.0/reference/vsl.html

const (
	// Client request header
	ReqHeader = "ReqHeader"
	// Client response header
	RespHeader = "RespHeader"
	// Backend request header
	BereqHeader = "BereqHeader"
	// Backend response header
	BerespHeader = "BerespHeader"
	// Object header
	ObjHeader = "ObjHeader"
)

const (
	// Request header unset
	ReqUnset = "ReqUnset"
	// Response header unset
	RespUnset = "RespUnset"
	// Backend request unset header
	BereqUnset = "BereqUnset"
	// Backend response unset header
	BerespUnset = "BerespUnset"
	// Object header unset
	ObjUnset = "ObjUnset"
)

// IsHeader check if the given tag represents an HTTP header
func IsHeader(tag string) bool {
	headers := []string{ReqHeader, RespHeader, BereqHeader, BerespHeader, ObjHeader}
	for _, v := range headers {
		if tag == v {
			return true
		}
	}
	return false
}

const (
	// Marks the start of a VXID
	Begin = "Begin"
	// Marks the end of a VXID
	End = "End"
	// Logged when a backend connection is closed
	BackendClose = "BackendClose"
	// Logged when a new backend connection is opened
	BackendOpen = "BackendOpen"
	// Logged when a backend connection is started
	BackendStart = "BackendStart"
	// Contains byte counters from backend request processing
	BereqAcct = "BereqAcct"
	// Backend request method
	BereqMethod = "BereqMethod"
	// Backend request protocol
	BereqProtocol = "BereqProtocol"
	// Backend request URL
	BereqURL = "BereqURL"
	// Backend response protocol
	BerespProtocol = "BerespProtocol"
	// Backend response reason
	BerespReason = "BerespReason"
	// Backend response status
	BerespStatus = "BerespStatus"
	// Bogus HTTP received
	BogoHeader = "BogoHeader"
	// ESI parser error or warning message
	ESI_xmlerror = "ESI_xmlerror"
	// Error messages
	Error = "Error"
	// Object evicted due to ban
	ExpBan = "ExpBan"
	// Object expiry event
	ExpKill = "ExpKill"
	// Error while fetching object
	FetchError = "FetchError"
	// Body fetched from backend
	Fetch_Body = "Fetch_Body"
	// Body filters
	Filters = "Filters"
	// G(un)zip performed on object
	Gzip = "Gzip"
	// Hit object in cache
	Hit = "Hit"
	// Hit for miss object in cache
	HitMiss = "HitMiss"
	// Hit for pass object in cache
	HitPass = "HitPass"
	// Unparsable HTTP request
	HttpGarbage = "HttpGarbage"
	// Size of object body
	Length = "Length"
	// Links to a child VXID
	Link = "Link"
	// Failed attempt to set HTTP header
	LostHeader = "LostHeader"
	// Informational messages about request handling
	Notice = "Notice"
	// Object protocol
	ObjProtocol = "ObjProtocol"
	// Object response
	ObjReason = "ObjReason"
	// Object status
	ObjStatus = "ObjStatus"
	// Pipe byte counts
	PipeAcct = "PipeAcct"
	// PROXY protocol information
	Proxy = "Proxy"
	// Unparseble PROXY request
	ProxyGarbage = "ProxyGarbage"
	// Request handling byte counts
	ReqAcct = "ReqAcct"
	// Client request method
	ReqMethod = "ReqMethod"
	// Client request protocol
	ReqProtocol = "ReqProtocol"
	// Client request start
	ReqStart = "ReqStart"
	// Client request URL
	ReqURL = "ReqURL"
	// Client response protocol
	RespProtocol = "RespProtocol"
	// Client response response
	RespReason = "RespReason"
	// Client response status
	RespStatus = "RespStatus"
	// Client connection closed
	SessClose = "SessClose"
	// Client connection accept failed
	SessError = "SessError"
	// Client connection opened
	SessOpen = "SessOpen"
	// Where object is stored
	Storage = "Storage"
	// TTL set on object
	TTL = "TTL"
	// Timing information
	Timestamp = "Timestamp"
	// VCL execution error message
	VCL_Error = "VCL_Error"
	// Log statement from VCL
	VCL_Log = "VCL_Log"
	// VCL ACL check results
	VCL_acl = "VCL_acl"
	// VCL method called
	VCL_call = "VCL_call"
	// VCL method return value
	VCL_return = "VCL_return"
	// VCL trace data
	VCL_trace = "VCL_trace"
	// VCL in use
	VCL_use = "VCL_use"
	// VSL API warnings and error message
	VSL = "VSL"
	// Fetch filter accounting
	VfpAcct = "VfpAcct"
)
