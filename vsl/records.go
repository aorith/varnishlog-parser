package vsl

import (
	"fmt"
	"net"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	LinkTypeSession = "sess"
	LinkTypeRequest = "req"
	LinkTypeBereq   = "bereq"
)

const (
	VCLCallRECV            = "RECV"
	VCLCallHASH            = "HASH"
	VCLCallPASS            = "PASS"
	VCLCallMISS            = "MISS"
	VCLCallHIT             = "HIT"
	VCLCallSYNTH           = "SYNTH"
	VCLCallDELIVER         = "DELIVER"
	VCLCallBACKENDRESPONSE = "BACKEND_RESPONSE"
	VCLCallBACKENDFETCH    = "BACKEND_FETCH"
	VCLCallBACKENDERROR    = "BACKEND_ERROR"
)

// Record interface for all the VSL log records
type Record interface {
	Tag() string
	Value() string
	RawLog() string
}

// BaseRecord is a single VSL log line split by tag and value
type BaseRecord struct {
	tag    string // Begin, Timestamp, ReqURL, ReqHeader, ...
	value  string
	rawLog string
}

func (r BaseRecord) Tag() string {
	return r.tag
}

func (r BaseRecord) Value() string {
	return r.value
}

func (r BaseRecord) RawLog() string {
	return r.rawLog
}

func NewBaseRecord(rawLog string) (BaseRecord, error) {
	fields := strings.Fields(rawLog)
	if len(fields) < 2 {
		return BaseRecord{}, fmt.Errorf("could not parse line %q", rawLog)
	}

	tag := fields[1] // e.g: Begin
	firstIndex := strings.Index(rawLog, tag)
	value := rawLog[firstIndex+len(tag):]
	value = strings.TrimLeft(value, " \t")

	return BaseRecord{tag: tag, value: value, rawLog: rawLog}, nil
}

// BeginRecord represents the start of a transaction log
type BeginRecord struct {
	BaseRecord
	recordType       string // sess, req, bereq, ...
	parentVXID       VXID
	esiLevel         int
	reasonOrProtocol string
}

func (r BeginRecord) Type() string {
	return r.recordType
}

func (r BeginRecord) ParentVXID() VXID {
	return r.parentVXID
}

func (r BeginRecord) ESILevel() int {
	return r.esiLevel
}

func (r BeginRecord) ReasonOrProtocol() string {
	return r.reasonOrProtocol
}

func NewBeginRecord(blr BaseRecord) (BeginRecord, error) {
	parts := strings.Fields(blr.Value())
	if len(parts) != 3 && len(parts) != 4 {
		return BeginRecord{}, fmt.Errorf("conversion to BeginRecord failed, incorrect len on line %q", blr.RawLog())
	}
	if len(parts) == 4 {
		if parts[2] != "esi" {
			return BeginRecord{}, fmt.Errorf("conversion to BeginRecord failed, len is 4 but it is not an ESI on line %q", blr.RawLog())
		}
		level, err := strconv.Atoi(parts[3])
		if err != nil {
			return BeginRecord{}, fmt.Errorf("conversion to BeginRecord failed, extraction of ESI level failed on line %q, error: %s", blr.RawLog(), err)
		}
		parentVXID, err := parseVXID(parts[1])
		if err != nil {
			return BeginRecord{}, fmt.Errorf("conversion to BeginRecord failed, bad VXID on line %q, error: %s", blr.RawLog(), err)
		}

		return BeginRecord{BaseRecord: blr, recordType: parts[0], parentVXID: parentVXID, esiLevel: level, reasonOrProtocol: parts[2]}, nil
	}

	parentVXID, err := parseVXID(parts[1])
	if err != nil {
		return BeginRecord{}, fmt.Errorf("conversion to BeginRecord failed, bad VXID on line %q, error: %s", blr.RawLog(), err)
	}

	return BeginRecord{BaseRecord: blr, recordType: parts[0], parentVXID: parentVXID, esiLevel: 0, reasonOrProtocol: parts[2]}, nil
}

// HeaderRecord interface
type HeaderRecord interface {
	Name() string
	Value() string
}

// headerRecord is a generic header record
type headerRecord struct {
	BaseRecord
	name  string
	value string
}

func (r headerRecord) Name() string {
	return r.name
}

func (r headerRecord) Value() string {
	return r.value
}

func newHeaderRecord(blr BaseRecord) (headerRecord, error) {
	fields := strings.SplitAfterN(blr.Value(), ":", 2)
	if len(fields) < 2 {
		return headerRecord{}, fmt.Errorf("conversion to HeaderRecord failed on line %q", blr.RawLog())
	}

	name := fields[0]
	firstIndex := strings.Index(blr.Value(), name)
	value := strings.TrimLeft(blr.Value()[firstIndex+len(name):], " \t")

	name = strings.TrimRight(name, ": \t")
	// Canonical format for the header key
	name = textproto.CanonicalMIMEHeaderKey(name)

	return headerRecord{
		BaseRecord: blr,
		name:       name,
		value:      value,
	}, nil
}

// ReqHeaderRecord holds headers for the vsl tag ReqHeader
type ReqHeaderRecord struct{ headerRecord }

func NewReqHeaderRecord(blr BaseRecord) (ReqHeaderRecord, error) {
	hr, err := newHeaderRecord(blr)
	if err != nil {
		return ReqHeaderRecord{}, fmt.Errorf("conversion to ReqHeaderRecord failed on line %q", blr.RawLog())
	}
	return ReqHeaderRecord{headerRecord: hr}, nil
}

// RespHeaderRecord holds headers for the vsl tag RespHeader
type RespHeaderRecord struct{ headerRecord }

func NewRespHeaderRecord(blr BaseRecord) (RespHeaderRecord, error) {
	hr, err := newHeaderRecord(blr)
	if err != nil {
		return RespHeaderRecord{}, fmt.Errorf("conversion to RespHeaderRecord failed on line %q", blr.RawLog())
	}
	return RespHeaderRecord{headerRecord: hr}, nil
}

// BereqHeaderRecord holds headers for the vsl tag BereqHeader
type BereqHeaderRecord struct{ headerRecord }

func NewBereqHeaderRecord(blr BaseRecord) (BereqHeaderRecord, error) {
	hr, err := newHeaderRecord(blr)
	if err != nil {
		return BereqHeaderRecord{}, fmt.Errorf("conversion to BereqHeaderRecord failed on line %q", blr.RawLog())
	}
	return BereqHeaderRecord{headerRecord: hr}, nil
}

// BerespHeaderRecord holds headers for the vsl tag BerespHeader
type BerespHeaderRecord struct{ headerRecord }

func NewBerespHeaderRecord(blr BaseRecord) (BerespHeaderRecord, error) {
	hr, err := newHeaderRecord(blr)
	if err != nil {
		return BerespHeaderRecord{}, fmt.Errorf("conversion to BerespHeaderRecord failed on line %q", blr.RawLog())
	}
	return BerespHeaderRecord{headerRecord: hr}, nil
}

// ObjHeaderRecord holds headers for the vsl tag ObjHeader
type ObjHeaderRecord struct{ headerRecord }

func NewObjHeaderRecord(blr BaseRecord) (ObjHeaderRecord, error) {
	hr, err := newHeaderRecord(blr)
	if err != nil {
		return ObjHeaderRecord{}, fmt.Errorf("conversion to ObjHeaderRecord failed on line %q", blr.RawLog())
	}
	return ObjHeaderRecord{headerRecord: hr}, nil
}

// ReqUnsetRecord holds a header name and value that is unset in VCL
type ReqUnsetRecord struct{ headerRecord }

func NewReqUnsetRecord(blr BaseRecord) (ReqUnsetRecord, error) {
	hr, err := newHeaderRecord(blr)
	if err != nil {
		return ReqUnsetRecord{}, fmt.Errorf("conversion to ReqUnsetRecord failed on line %q", blr.RawLog())
	}
	return ReqUnsetRecord{headerRecord: hr}, nil
}

// BereqUnsetRecord holds a header name and value that is unset in VCL
type BereqUnsetRecord struct{ headerRecord }

func NewBereqUnsetRecord(blr BaseRecord) (BereqUnsetRecord, error) {
	hr, err := newHeaderRecord(blr)
	if err != nil {
		return BereqUnsetRecord{}, fmt.Errorf("conversion to BereqUnsetRecord failed on line %q", blr.RawLog())
	}
	return BereqUnsetRecord{headerRecord: hr}, nil
}

// RespUnsetRecord holds a header name and value that is unset in VCL
type RespUnsetRecord struct{ headerRecord }

func NewRespUnsetRecord(blr BaseRecord) (RespUnsetRecord, error) {
	hr, err := newHeaderRecord(blr)
	if err != nil {
		return RespUnsetRecord{}, fmt.Errorf("conversion to RespUnsetRecord failed on line %q", blr.RawLog())
	}
	return RespUnsetRecord{headerRecord: hr}, nil
}

// BerespUnsetRecord holds a header name and value that is unset in VCL
type BerespUnsetRecord struct{ headerRecord }

func NewBerespUnsetRecord(blr BaseRecord) (BerespUnsetRecord, error) {
	hr, err := newHeaderRecord(blr)
	if err != nil {
		return BerespUnsetRecord{}, fmt.Errorf("conversion to BerespUnsetRecord failed on line %q", blr.RawLog())
	}
	return BerespUnsetRecord{headerRecord: hr}, nil
}

// ObjUnsetRecord holds a header name and value that is unset in VCL
type ObjUnsetRecord struct{ headerRecord }

func NewObjUnsetRecord(blr BaseRecord) (ObjUnsetRecord, error) {
	hr, err := newHeaderRecord(blr)
	if err != nil {
		return ObjUnsetRecord{}, fmt.Errorf("conversion to ObjUnsetRecord failed on line %q", blr.RawLog())
	}
	return ObjUnsetRecord{headerRecord: hr}, nil
}

// BackendOpenRecord holds information about a new backend connection
type BackendOpenRecord struct {
	BaseRecord
	fileDescriptor int
	name           string
	remoteAddr     net.IP
	remotePort     int
	localAddr      net.IP
	localPort      int
	reason         string // connect, reuse
}

func (r BackendOpenRecord) String() string {
	return fmt.Sprintf("%s (%s:%d) %s", r.name, r.remoteAddr.String(), r.remotePort, r.reason)
}

func (r BackendOpenRecord) FileDescriptor() int {
	return r.fileDescriptor
}

func (r BackendOpenRecord) Name() string {
	return r.name
}

func (r BackendOpenRecord) RemoteAddr() net.IP {
	return r.remoteAddr
}

func (r BackendOpenRecord) RemotePort() int {
	return r.remotePort
}

func (r BackendOpenRecord) LocalAddr() net.IP {
	return r.localAddr
}

func (r BackendOpenRecord) LocalPort() int {
	return r.localPort
}

func (r BackendOpenRecord) Reason() string {
	return r.reason
}

func NewBackendOpenRecord(blr BaseRecord) (BackendOpenRecord, error) {
	parts := strings.Fields(blr.Value())
	if len(parts) < 6 {
		return BackendOpenRecord{}, fmt.Errorf("conversion to BackendOpenRecord failed, incorrect len on line %q", blr.RawLog())
	}

	fileDesc, err := strconv.Atoi(parts[0])
	if err != nil {
		return BackendOpenRecord{}, fmt.Errorf("conversion to BackendOpenRecord failed, bad file descriptor on line %q", blr.RawLog())
	}

	remoteAddr := net.ParseIP(parts[2])
	if remoteAddr == nil {
		return BackendOpenRecord{}, fmt.Errorf("conversion to BackendOpenRecord failed, bad remoteAddr on line %q", blr.RawLog())
	}

	remotePort, err := strconv.Atoi(parts[3])
	if err != nil {
		return BackendOpenRecord{}, fmt.Errorf("conversion to BackendOpenRecord failed, bad remotePort on line %q", blr.RawLog())
	}

	localAddr := net.ParseIP(parts[4])
	if localAddr == nil {
		return BackendOpenRecord{}, fmt.Errorf("conversion to BackendOpenRecord failed, bad localAddr on line %q", blr.RawLog())
	}

	localPort, err := strconv.Atoi(parts[5])
	if err != nil {
		return BackendOpenRecord{}, fmt.Errorf("conversion to BackendOpenRecord failed, bad localPort on line %q", blr.RawLog())
	}

	reason := "-"
	if len(parts) >= 7 {
		reason = parts[6]

	}

	return BackendOpenRecord{
		BaseRecord:     blr,
		fileDescriptor: fileDesc,
		name:           parts[1],
		remoteAddr:     remoteAddr,
		remotePort:     remotePort,
		localAddr:      localAddr,
		localPort:      localPort,
		reason:         reason,
	}, nil
}

// BackendStartRecord holds information about a new backend connection
type BackendStartRecord struct {
	BaseRecord
	remoteAddr net.IP
	remotePort int
}

func (r BackendStartRecord) RemoteAddr() net.IP {
	return r.remoteAddr
}

func (r BackendStartRecord) RemotePort() int {
	return r.remotePort
}

func NewBackendStartRecord(blr BaseRecord) (BackendStartRecord, error) {
	parts := strings.Fields(blr.Value())
	if len(parts) < 2 {
		return BackendStartRecord{}, fmt.Errorf("conversion to BackendStartRecord failed, incorrect len on line %q", blr.RawLog())
	}

	remoteAddr := net.ParseIP(parts[0])
	if remoteAddr == nil {
		return BackendStartRecord{}, fmt.Errorf("conversion to BackendStartRecord failed, bad remoteAddr on line %q", blr.RawLog())
	}

	remotePort, err := strconv.Atoi(parts[1])
	if err != nil {
		return BackendStartRecord{}, fmt.Errorf("conversion to BackendStartRecord failed, bad remotePort on line %q", blr.RawLog())
	}

	return BackendStartRecord{
		BaseRecord: blr,
		remoteAddr: remoteAddr,
		remotePort: remotePort,
	}, nil
}

// BackendCloseRecord holds information about a backend connection close
type BackendCloseRecord struct {
	BaseRecord
	fileDescriptor int
	name           string
	reason         string
}

func (r BackendCloseRecord) FileDescriptor() int {
	return r.fileDescriptor
}

func (r BackendCloseRecord) Name() string {
	return r.name
}

func (r BackendCloseRecord) Reason() string {
	return r.reason
}

func NewBackendCloseRecord(blr BaseRecord) (BackendCloseRecord, error) {
	parts := strings.Fields(blr.Value())
	if len(parts) < 2 {
		return BackendCloseRecord{}, fmt.Errorf("conversion to BackendCloseRecord failed, incorrect len on line %q", blr.RawLog())
	}

	fileDesc, err := strconv.Atoi(parts[0])
	if err != nil {
		return BackendCloseRecord{}, fmt.Errorf("conversion to BackendCloseRecord failed, bad file descriptor on line %q", blr.RawLog())
	}

	reason := "unknown"
	if len(parts) >= 3 {
		reason = parts[2]
	}

	return BackendCloseRecord{
		BaseRecord:     blr,
		fileDescriptor: fileDesc,
		name:           parts[1],
		reason:         reason,
	}, nil
}

// AcctRecord holds accounting information for ReqAcct and BereqAcct tags
type AcctRecord struct {
	BaseRecord
	headerTx SizeValue // Header bytes transmitted
	bodyTx   SizeValue // Body bytes transmitted
	totalTx  SizeValue // Total bytes transmitted
	headerRx SizeValue // Header bytes received
	bodyRx   SizeValue // Body bytes received
	totalRx  SizeValue // Total bytes received
}

func (r AcctRecord) String() string {
	return fmt.Sprintf(
		"Tx(hdr %s, body %s, total %s) | Rx(hdr %s, body %s, total %s)",
		r.headerTx.String(),
		r.bodyTx.String(),
		r.totalTx.String(),
		r.headerRx.String(),
		r.bodyRx.String(),
		r.totalRx.String(),
	)
}

func (r AcctRecord) HeaderTx() SizeValue {
	return r.headerTx
}

func (r AcctRecord) BodyTx() SizeValue {
	return r.bodyTx
}

func (r AcctRecord) TotalTx() SizeValue {
	return r.totalTx
}

func (r AcctRecord) HeaderRx() SizeValue {
	return r.headerRx
}

func (r AcctRecord) BodyRx() SizeValue {
	return r.bodyRx
}

func (r AcctRecord) TotalRx() SizeValue {
	return r.totalRx
}

func NewAcctRecord(blr BaseRecord) (AcctRecord, error) {
	parts := strings.Fields(blr.Value())
	if len(parts) != 6 {
		return AcctRecord{}, fmt.Errorf("conversion to AcctRecord failed, incorrect len on line %q", blr.RawLog())
	}

	record := AcctRecord{BaseRecord: blr}
	fields := []*SizeValue{
		&record.headerTx,
		&record.bodyTx,
		&record.totalTx,
		&record.headerRx,
		&record.bodyRx,
		&record.totalRx,
	}

	for i := range fields {
		value, err := strconv.Atoi(parts[i])
		if err != nil {
			return AcctRecord{}, fmt.Errorf("conversion to AcctRecord failed, bad value in part[%d] on line %q", i, blr.RawLog())
		}
		*fields[i] = SizeValue(value)
	}

	return record, nil
}

type TimestampRecord struct {
	BaseRecord
	eventLabel string        // Start, Req, Fetch, Process, Resp, ...
	absolute   time.Time     // Absolute time of the timestamp
	sinceStart time.Duration // Duration since the start of the tx
	sinceLast  time.Duration // Duration since the last timestamp
}

func (r TimestampRecord) String() string {
	return fmt.Sprintf(
		"%s | Elapsed: %s | Total: %s",
		r.eventLabel, r.sinceLast.String(), r.sinceStart.String(),
	)
}

func (r TimestampRecord) EventLabel() string {
	return r.eventLabel
}

func (r TimestampRecord) AbsoluteTime() time.Time {
	return r.absolute
}

func (r TimestampRecord) SinceStart() time.Duration {
	return r.sinceStart
}

func (r TimestampRecord) SinceLast() time.Duration {
	return r.sinceLast
}

func NewTimestampRecord(blr BaseRecord) (TimestampRecord, error) {
	parts := strings.Fields(blr.Value())
	if len(parts) != 4 {
		return TimestampRecord{}, fmt.Errorf("conversion to TimestampRecord failed, incorrect len on line %q", blr.RawLog())
	}

	ab, err := convertToUnixTimestamp(parts[1])
	if err != nil {
		return TimestampRecord{}, fmt.Errorf("conversion to TimestampRecord failed, bad field absolute time on line %q", blr.RawLog())
	}

	sinceStart, err := convertStrToDuration(parts[2], time.Second)
	if err != nil {
		return TimestampRecord{}, fmt.Errorf("conversion to TimestampRecord failed, bad field since start on line %q", blr.RawLog())
	}
	sinceLast, err := convertStrToDuration(parts[3], time.Second)
	if err != nil {
		return TimestampRecord{}, fmt.Errorf("conversion to TimestampRecord failed, bad field since last on line %q", blr.RawLog())
	}

	return TimestampRecord{
		BaseRecord: blr,
		eventLabel: strings.TrimRight(parts[0], ":"),
		absolute:   ab,
		sinceStart: sinceStart,
		sinceLast:  sinceLast,
	}, nil
}

// ReqStartRecord holds information about the start of request processing
type ReqStartRecord struct {
	BaseRecord
	clientIP   net.IP
	clientPort int
	listener   string // Listener name (from -a)
}

func (r ReqStartRecord) ClientIP() net.IP {
	return r.clientIP
}

func (r ReqStartRecord) ClientPort() int {
	return r.clientPort
}

func (r ReqStartRecord) Listener() string {
	return r.listener
}

func NewReqStartRecord(blr BaseRecord) (ReqStartRecord, error) {
	parts := strings.Fields(blr.Value())
	if len(parts) != 3 {
		return ReqStartRecord{}, fmt.Errorf("conversion to ReqStartRecord failed, incorrect len on line %q", blr.RawLog())
	}

	clientIP := net.ParseIP(parts[0])
	if clientIP == nil {
		return ReqStartRecord{}, fmt.Errorf("conversion to BackendOpenRecord failed, bad clientAddr on line %q", blr.RawLog())
	}

	clientPort, err := strconv.Atoi(parts[1])
	if err != nil {
		return ReqStartRecord{}, fmt.Errorf("conversion to BackendOpenRecord failed, bad clientPort on line %q", blr.RawLog())
	}

	return ReqStartRecord{BaseRecord: blr, clientIP: clientIP, clientPort: clientPort, listener: parts[2]}, nil
}

// LinkRecord Links to a child transaction
type LinkRecord struct {
	BaseRecord
	txid     string // {vxid}_{type}[_{esiLevel}]
	txType   string // sess, req, bereq, ...
	vxid     VXID
	reason   string
	esiLevel int
}

func (r LinkRecord) TXID() string {
	return r.txid
}

func (r LinkRecord) Type() string {
	return r.txType
}

func (r LinkRecord) VXID() VXID {
	return r.vxid
}

func (r LinkRecord) ESILevel() int {
	return r.esiLevel
}

func (r LinkRecord) Reason() string {
	return r.reason
}

func NewLinkRecord(blr BaseRecord) (LinkRecord, error) {
	parts := strings.Fields(blr.Value())
	if len(parts) != 3 && len(parts) != 4 {
		return LinkRecord{}, fmt.Errorf("conversion to LinkRecord failed, incorrect len on line %q", blr.RawLog())
	}
	if len(parts) == 4 {
		if parts[2] != "esi" {
			return LinkRecord{}, fmt.Errorf("conversion to LinkRecord failed, len is 4 but it is not an ESI on line %q", blr.RawLog())
		}
		level, err := strconv.Atoi(parts[3])
		if err != nil {
			return LinkRecord{}, fmt.Errorf("conversion to LinkRecord failed, extraction of ESI level failed on line %q, error: %s", blr.RawLog(), err)
		}
		vxid, err := parseVXID(parts[1])
		if err != nil {
			return LinkRecord{}, fmt.Errorf("conversion to LinkRecord failed, bad VXID on line %q, error: %s", blr.RawLog(), err)
		}

		return LinkRecord{
			BaseRecord: blr,
			txid:       parseTXID(vxid, parts[0], level),
			txType:     parts[0],
			vxid:       vxid,
			esiLevel:   level,
			reason:     parts[2],
		}, nil
	}

	vxid, err := parseVXID(parts[1])
	if err != nil {
		return LinkRecord{}, fmt.Errorf("conversion to LinkRecord failed, bad VXID on line %q, error: %s", blr.RawLog(), err)
	}

	return LinkRecord{
		BaseRecord: blr,
		txid:       parseTXID(vxid, parts[0], 0),
		txType:     parts[0],
		vxid:       vxid,
		esiLevel:   0,
		reason:     parts[2],
	}, nil
}

// URLRecord holds request URL from ReqURL and BereqURL tags
type URLRecord struct {
	BaseRecord
	url url.URL
}

func (r URLRecord) URL() url.URL {
	return r.url
}

func (r URLRecord) Path() string {
	return r.url.Path
}

func (r URLRecord) Query() string {
	return r.url.Query().Encode()
}

func NewURLRecord(blr BaseRecord) (URLRecord, error) {
	url, err := url.Parse(blr.Value())
	if err != nil {
		return URLRecord{}, fmt.Errorf("conversion to URLRecord failed, could not parse URL on line %q", blr.RawLog())
	}

	return URLRecord{BaseRecord: blr, url: *url}, nil
}

// FiltersRecord holds the list of filters applied to the body
type FiltersRecord struct {
	BaseRecord
	filters []string
}

func (r FiltersRecord) Filters() []string {
	return r.filters
}

func NewFiltersRecord(blr BaseRecord) (FiltersRecord, error) {
	return FiltersRecord{BaseRecord: blr, filters: strings.Fields(blr.Value())}, nil
}

// StatusRecord represents an HTTP code response status
type StatusRecord struct {
	BaseRecord
	status int
}

func (r StatusRecord) Status() int {
	return r.status
}

func NewStatusRecord(blr BaseRecord) (StatusRecord, error) {
	v, err := strconv.Atoi(blr.Value())
	if err != nil {
		return StatusRecord{}, fmt.Errorf("conversion to StatusRecord failed, bad field status on line %q", blr.RawLog())
	}
	return StatusRecord{BaseRecord: blr, status: v}, nil
}

// LengthRecord represents the size of a fetch body
type LengthRecord struct {
	BaseRecord
	size SizeValue
}

func (r LengthRecord) Size() SizeValue {
	return r.size
}

func NewLengthRecord(blr BaseRecord) (LengthRecord, error) {
	size, err := strconv.Atoi(blr.Value())
	if err != nil {
		return LengthRecord{}, fmt.Errorf("conversion to LengthRecord failed, bad size value on line %q", blr.RawLog())
	}
	return LengthRecord{BaseRecord: blr, size: SizeValue(size)}, nil
}

// HitRecord contains information about a hit of an object in the cache
type HitRecord struct {
	BaseRecord
	objVXID VXID          // object VXID
	ttl     time.Duration // remaining TTL
	grace   time.Duration // grace period
	keep    time.Duration // keep period
}

func (r HitRecord) String() string {
	return fmt.Sprintf(
		"%d | TTL: %s | Grace: %s | Keep: %s",
		r.objVXID,
		r.ttl.String(),
		r.grace.String(),
		r.keep.String(),
	)
}

func (r HitRecord) ObjVXID() VXID {
	return r.objVXID
}

func (r HitRecord) TTL() time.Duration {
	return r.ttl
}

func (r HitRecord) Grace() time.Duration {
	return r.grace
}

func (r HitRecord) Keep() time.Duration {
	return r.keep
}

func NewHitRecord(blr BaseRecord) (HitRecord, error) {
	parts := strings.Fields(blr.Value())
	if len(parts) < 4 {
		return HitRecord{}, fmt.Errorf("conversion to HitRecord failed, incorrect len on line %q", blr.RawLog())
	}

	vxid, err := parseVXID(parts[0])
	if err != nil {
		return HitRecord{}, fmt.Errorf("conversion to HitRecord failed, bad VXID on line %q", blr.RawLog())
	}

	ttl, err := convertStrToDuration(parts[1], time.Second)
	if err != nil {
		return HitRecord{}, fmt.Errorf("conversion to HitRecord failed, bad field TTL on line %q", blr.RawLog())
	}

	grace, err := convertStrToDuration(parts[2], time.Second)
	if err != nil {
		return HitRecord{}, fmt.Errorf("conversion to HitRecord failed, bad field grace on line %q", blr.RawLog())
	}

	keep, err := convertStrToDuration(parts[3], time.Second)
	if err != nil {
		return HitRecord{}, fmt.Errorf("conversion to HitRecord failed, bad field keep on line %q", blr.RawLog())
	}

	return HitRecord{BaseRecord: blr, objVXID: vxid, ttl: ttl, grace: grace, keep: keep}, nil
}

// TTLRecord reprensets the ttl, grace, keep values for an object
type TTLRecord struct {
	BaseRecord
	source      string        // "RFC", "VCL" or "HFP"
	ttl         time.Duration // Time-to-live
	grace       time.Duration // Grace period
	keep        time.Duration // Keep period
	reference   time.Time     // Reference time for TTL
	age         time.Time     // Age (incl Age: header value)
	date        time.Time     // Date header
	expires     time.Time     // Expires header
	maxAge      time.Duration // Max-Age from Cache-Control header
	cacheStatus string        // "cacheable" or "uncacheable"
}

func (r TTLRecord) String() string {
	if r.Source() == "RFC" {
		return fmt.Sprintf(
			"%s | TTL %s, Grace %s, Keep %s, Reference %d, Age %d, Date %d, Expires %d, Max-Age %s | %s",
			r.source,
			r.ttl.String(),
			r.grace.String(),
			r.keep.String(),
			r.reference.Unix(),
			r.age.Unix(),
			r.date.Unix(),
			r.expires.Unix(),
			r.maxAge.String(),
			r.cacheStatus,
		)
	}

	return fmt.Sprintf(
		"%s | TTL %s, Grace %s, Keep %s, Reference %d | %s",
		r.Source(),
		r.TTL().String(),
		r.Grace().String(),
		r.Keep().String(),
		r.Reference().Unix(),
		r.CacheStatus(),
	)
}

func (r TTLRecord) Source() string {
	return r.source
}

func (r TTLRecord) TTL() time.Duration {
	return r.ttl
}

func (r TTLRecord) Grace() time.Duration {
	return r.grace
}

func (r TTLRecord) Keep() time.Duration {
	return r.keep
}

func (r TTLRecord) Reference() time.Time {
	return r.reference
}

func (r TTLRecord) Age() time.Time {
	return r.age
}

func (r TTLRecord) Date() time.Time {
	return r.date
}

func (r TTLRecord) Expires() time.Time {
	return r.expires
}

func (r TTLRecord) MaxAge() time.Duration {
	return r.maxAge
}

func (r TTLRecord) CacheStatus() string {
	return r.cacheStatus
}

func NewTTLRecord(blr BaseRecord) (TTLRecord, error) {
	// RFC 120 10 0 1606398419 1606398419 1606398419 0 0 cacheable
	// VCL 120 10 0 1606400537 uncacheable
	// HFP 10 0 0 1606402666 uncacheable
	parts := strings.Fields(blr.Value())
	if len(parts) < 6 {
		return TTLRecord{}, fmt.Errorf("conversion to TTLRecord failed, incorrect len on line %q", blr.RawLog())
	}

	r := TTLRecord{BaseRecord: blr, source: parts[0]}

	// First 5 parts are common
	ttl, err := strconv.Atoi(parts[1])
	if err != nil {
		return r, fmt.Errorf("conversion to TTLRecord failed, bad field ttl on line %q", blr.RawLog())
	}
	r.ttl = time.Duration(ttl * int(time.Second))

	grace, err := strconv.Atoi(parts[2])
	if err != nil {
		return r, fmt.Errorf("conversion to TTLRecord failed, bad field grace on line %q", blr.RawLog())
	}
	r.grace = time.Duration(grace * int(time.Second))

	keep, err := strconv.Atoi(parts[3])
	if err != nil {
		return r, fmt.Errorf("conversion to TTLRecord failed, bad field keep on line %q", blr.RawLog())
	}
	r.keep = time.Duration(keep * int(time.Second))

	ref, err := convertToUnixTimestamp(parts[4])
	if err != nil {
		return r, fmt.Errorf("conversion to TTLRecord failed, bad field reference on line %q", blr.RawLog())
	}
	r.reference = ref

	// Check if we are parsing a VCL or HFP source (6 fields) or a HFP
	if len(parts) == 6 {
		r.cacheStatus = parts[5]
		return r, nil
	}

	if len(parts) != 10 {
		return TTLRecord{}, fmt.Errorf("conversion to TTLRecord failed, incorrect len (wanted 10) on line %q", blr.RawLog())
	}

	age, err := convertToUnixTimestamp(parts[5])
	if err != nil {
		return r, fmt.Errorf("conversion to TTLRecord failed, bad field age on line %q", blr.RawLog())
	}
	r.age = age

	date, err := convertToUnixTimestamp(parts[6])
	if err != nil {
		return r, fmt.Errorf("conversion to TTLRecord failed, bad field date on line %q", blr.RawLog())
	}
	r.date = date

	expires, err := convertToUnixTimestamp(parts[7])
	if err != nil {
		return r, fmt.Errorf("conversion to TTLRecord failed, bad field expires on line %q", blr.RawLog())
	}
	r.expires = expires

	maxAge, err := strconv.Atoi(parts[8])
	if err != nil {
		return r, fmt.Errorf("conversion to TTLRecord failed, bad field maxAge on line %q", blr.RawLog())
	}
	r.maxAge = time.Duration(maxAge * int(time.Second))

	// Last field
	r.cacheStatus = parts[9]

	return r, nil
}

// VCLLogRecord holds vsl tag VCL_Log
// key is empty if the log value is not formatted as 'key: value'
type VCLLogRecord struct {
	BaseRecord
	key   string // Only if the format of the log is 'Key: value'
	value string
}

func (r VCLLogRecord) String() string {
	if r.key != "" {
		return r.key + ": " + r.value
	}
	return r.value
}

func (r VCLLogRecord) Key() string {
	return r.key
}

func (r VCLLogRecord) Value() string {
	return r.value
}

func NewVCLLogRecord(blr BaseRecord) (VCLLogRecord, error) {
	fields := strings.SplitAfterN(blr.Value(), ":", 2)
	if len(fields) < 2 {
		return VCLLogRecord{BaseRecord: blr, key: "", value: blr.Value()}, nil
	}

	key := fields[0]
	firstIndex := strings.Index(blr.Value(), key)
	value := strings.TrimLeft(blr.Value()[firstIndex+len(key):], " \t")
	return VCLLogRecord{BaseRecord: blr, key: strings.TrimRight(key, ": \t"), value: value}, nil
}

// StorageRecord holds the type and name of the storage backend the object is stored in
type StorageRecord struct {
	BaseRecord
	storageType string // malloc, file, persistent, ...
	name        string
}

func (r StorageRecord) Type() string {
	return r.storageType
}

func (r StorageRecord) Name() string {
	return r.name
}

func NewStorageRecord(blr BaseRecord) (StorageRecord, error) {
	parts := strings.Fields(blr.Value())
	if len(parts) < 2 {
		return StorageRecord{}, fmt.Errorf("conversion to StorageRecord failed, incorrect len on line %q", blr.RawLog())
	}

	return StorageRecord{BaseRecord: blr, storageType: parts[0], name: parts[1]}, nil
}

// FetchBodyRecord holds information about the mode to fetch the object from the backend
type FetchBodyRecord struct {
	BaseRecord
	mode   int    // Body fetch mode
	desc   string // Description of body fetch mode
	stream bool
}

func (r FetchBodyRecord) Mode() int {
	return r.mode
}

func (r FetchBodyRecord) Description() string {
	return r.desc
}

func (r FetchBodyRecord) IsStream() bool {
	return r.stream
}

func NewFetchBodyRecord(blr BaseRecord) (FetchBodyRecord, error) {
	parts := strings.Fields(blr.Value())
	if len(parts) != 3 {
		return FetchBodyRecord{}, fmt.Errorf("conversion to FetchBodyRecord failed, incorrect len on line %q", blr.RawLog())
	}

	m, err := strconv.Atoi(parts[0])
	if err != nil {
		return FetchBodyRecord{}, fmt.Errorf("conversion to FetchBodyRecord failed, bad field mode on line %q", blr.RawLog())
	}

	stream := false
	if parts[2] == "stream" {
		stream = true
	} else if parts[2] != "-" {
		return FetchBodyRecord{}, fmt.Errorf("conversion to FetchBodyRecord failed, unknown value for stream on line %q", blr.RawLog())
	}

	return FetchBodyRecord{BaseRecord: blr, mode: m, desc: parts[1], stream: stream}, nil
}

// SessOpenRecord is the first record for a client connection
// with the socket-endpoints of the connection
type SessOpenRecord struct {
	BaseRecord
	remoteAddr     net.IP
	remotePort     int
	socketName     string
	localAddr      net.IP
	localPort      int
	sessionStart   time.Time
	fileDescriptor int
}

func (r SessOpenRecord) String() string {
	return fmt.Sprintf(
		"%s:%d %s %s:%d (%s) %d",
		r.remoteAddr,
		r.remotePort,
		r.socketName,
		r.localAddr,
		r.localPort,
		r.sessionStart.UTC(),
		r.fileDescriptor,
	)
}

func (r SessOpenRecord) RemoteAddr() net.IP {
	return r.remoteAddr
}

func (r SessOpenRecord) RemotePort() int {
	return r.remotePort
}

func (r SessOpenRecord) SocketName() string {
	return r.socketName
}

func (r SessOpenRecord) LocalAddr() net.IP {
	return r.localAddr
}

func (r SessOpenRecord) LocalPort() int {
	return r.localPort
}

func (r SessOpenRecord) SessionStart() time.Time {
	return r.sessionStart
}

func (r SessOpenRecord) FileDescriptor() int {
	return r.fileDescriptor
}

func NewSessOpenRecord(blr BaseRecord) (SessOpenRecord, error) {
	parts := strings.Fields(blr.Value())
	if len(parts) != 7 {
		return SessOpenRecord{}, fmt.Errorf("conversion to SessOpenRecord failed, incorrect len on line %q", blr.RawLog())
	}

	remoteAddr := net.ParseIP(parts[0])
	if remoteAddr == nil {
		return SessOpenRecord{}, fmt.Errorf("conversion to SessOpenRecord failed, bad remoteAddr on line %q", blr.RawLog())
	}

	remotePort, err := strconv.Atoi(parts[1])
	if err != nil {
		return SessOpenRecord{}, fmt.Errorf("conversion to SessOpenRecord failed, bad remotePort on line %q", blr.RawLog())
	}

	localAddr := net.ParseIP(parts[3])
	if localAddr == nil {
		return SessOpenRecord{}, fmt.Errorf("conversion to SessOpenRecord failed, bad localAddr on line %q", blr.RawLog())
	}

	localPort, err := strconv.Atoi(parts[4])
	if err != nil {
		return SessOpenRecord{}, fmt.Errorf("conversion to SessOpenRecord failed, bad localPort on line %q", blr.RawLog())
	}

	sessionStart, err := convertToUnixTimestamp(parts[5])
	if err != nil {
		return SessOpenRecord{}, fmt.Errorf("conversion to SessOpenRecord failed, bad field sessionStart on line %q", blr.RawLog())
	}

	fileDesc, err := strconv.Atoi(parts[6])
	if err != nil {
		return SessOpenRecord{}, fmt.Errorf("conversion to SessOpenRecord failed, bad file descriptor on line %q", blr.RawLog())
	}

	return SessOpenRecord{
		BaseRecord:     blr,
		remoteAddr:     remoteAddr,
		remotePort:     remotePort,
		socketName:     parts[2],
		localAddr:      localAddr,
		localPort:      localPort,
		sessionStart:   sessionStart,
		fileDescriptor: fileDesc,
	}, nil
}

// SessCloseRecord is the last record for any client connection
type SessCloseRecord struct {
	BaseRecord
	reason   string
	duration time.Duration
}

func (r SessCloseRecord) String() string {
	return r.reason + " " + r.duration.String()
}

func (r SessCloseRecord) Reason() string {
	return r.reason
}

func (r SessCloseRecord) Duration() time.Duration {
	return r.duration
}

func NewSessCloseRecord(blr BaseRecord) (SessCloseRecord, error) {
	parts := strings.Fields(blr.Value())
	if len(parts) != 2 {
		return SessCloseRecord{}, fmt.Errorf("conversion to SessCloseRecord failed, invalid len on line %q", blr.RawLog())
	}
	d, err := convertStrToDuration(parts[1], time.Second)
	if err != nil {
		return SessCloseRecord{}, fmt.Errorf("conversion to SessCloseRecord failed, bad field duration on line %q", blr.RawLog())
	}
	return SessCloseRecord{BaseRecord: blr, reason: parts[0], duration: d}, nil
}

// GzipRecord holds G(un)zip performed on object
type GzipRecord struct {
	BaseRecord
	action                    string // G: Gzip, U: Gunzip, u: Gunzip-test
	when                      string // F: Fetch, D: Deliver
	object                    string // E: ESI, -: Plain object
	inputBytes                SizeValue
	outputBytes               SizeValue
	bitLocFirst               int64
	bitLocLast                int64
	bitLengthOfCompressedData int64
}

func (r GzipRecord) String() string {
	action := r.action
	switch r.action {
	case "G":
		action = "Gzip"
	case "U":
		action = "Gunzip"
	case "u":
		action = "Gunzip-test"
	}

	when := r.when
	switch r.when {
	case "F":
		when = "Fetch"
	case "D":
		when = "Deliver"
	}

	object := r.object
	switch r.object {
	case "E":
		object = "ESI"
	case "-":
		object = "Plain"
	}

	return fmt.Sprintf(
		"%s on %s for %s object | %s input | %s output | %d %d %d",
		action,
		when,
		object,
		r.inputBytes.String(),
		r.outputBytes.String(),
		r.bitLocFirst,
		r.bitLocLast,
		r.bitLengthOfCompressedData,
	)
}

func (r GzipRecord) Action() string {
	return r.action
}

func (r GzipRecord) When() string {
	return r.when
}

func (r GzipRecord) Object() string {
	return r.object
}

func (r GzipRecord) InputBytes() SizeValue {
	return r.inputBytes
}

func (r GzipRecord) OutputBytes() SizeValue {
	return r.outputBytes
}

func (r GzipRecord) BitLocFirst() int64 {
	return r.bitLocFirst
}

func (r GzipRecord) BitLocLast() int64 {
	return r.bitLocLast
}

func (r GzipRecord) BitLengthOfCompressedData() int64 {
	return r.bitLengthOfCompressedData
}

func NewGzipRecord(blr BaseRecord) (GzipRecord, error) {
	parts := strings.Fields(blr.Value())
	if len(parts) != 8 {
		return GzipRecord{}, fmt.Errorf("conversion to GzipRecord failed, incorrect len on line %q", blr.RawLog())
	}

	record := GzipRecord{BaseRecord: blr}

	record.action = parts[0]
	record.when = parts[1]
	record.object = parts[2]

	inputBytes, err := strconv.Atoi(parts[3])
	if err != nil {
		return GzipRecord{}, fmt.Errorf("conversion to GzipRecord failed, bad value for inputBytes on line %q", blr.RawLog())
	}
	record.inputBytes = SizeValue(inputBytes)

	outputBytes, err := strconv.Atoi(parts[4])
	if err != nil {
		return GzipRecord{}, fmt.Errorf("conversion to GzipRecord failed, bad value for outputBytes on line %q", blr.RawLog())
	}
	record.outputBytes = SizeValue(outputBytes)

	bitLocFirst, err := strconv.ParseInt(parts[5], 10, 64)
	if err != nil {
		return GzipRecord{}, fmt.Errorf("conversion to GzipRecord failed, bad value for bitLocFirst on line %q", blr.RawLog())
	}
	record.bitLocFirst = bitLocFirst

	bitLocLast, err := strconv.ParseInt(parts[6], 10, 64)
	if err != nil {
		return GzipRecord{}, fmt.Errorf("conversion to GzipRecord failed, bad value for bitLocLast on line %q", blr.RawLog())
	}
	record.bitLocLast = bitLocLast

	bitLengthOfCompressedData, err := strconv.ParseInt(parts[7], 10, 64)
	if err != nil {
		return GzipRecord{}, fmt.Errorf("conversion to GzipRecord failed, bad value for bitLengthOfCompressedData on line %q", blr.RawLog())
	}
	record.bitLengthOfCompressedData = bitLengthOfCompressedData

	return record, nil
}

/* BaseRecord aliases */

// EndRecord marks the end of a transaction
type EndRecord struct{ BaseRecord }

// VCLCallRecord is the VCL method called (RECV, DELIVER, BACKEND_FETCH, ...)
type VCLCallRecord struct{ BaseRecord }

// VCLReturnRecord is the VCL method return value (hash, lookup, fetch, deliver, ...)
type VCLReturnRecord struct{ BaseRecord }

// VCLUseRecord is the VCL name in use
type VCLUseRecord struct{ BaseRecord }

// ReasonRecord is the response reason
type ReasonRecord struct{ BaseRecord }

// FetchErrorRecord holds the error msg of an error while fetching the object from the backend
type FetchErrorRecord struct{ BaseRecord }

// MethodRecord holds the method for ReqMethod or BereqMethod tags
type MethodRecord struct{ BaseRecord }

// ProtocolRecord holds the protocol for the ReqProtocol, RespProtocol, BereqProtocol, ... tags
type ProtocolRecord struct{ BaseRecord }

// ErrorRecord holds error messages
type ErrorRecord struct{ BaseRecord }
