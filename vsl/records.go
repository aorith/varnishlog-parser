// SPDX-License-Identifier: MIT

package vsl

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/aorith/varnishlog-parser/vsl/tags"
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
	String() string
	GetTag() string
	GetRawValue() string
	GetRawLog() string
}

// BaseRecord is a single VSL log line split by tag and value
type BaseRecord struct {
	Tag      string // VSL Tag (Begin, Timestamp, ReqURL, ReqHeader, ...)
	RawValue string // Value after the tag

	rawLog string // Raw log line
}

func (r BaseRecord) String() string {
	return r.Tag + " " + r.RawValue
}

func (r BaseRecord) GetTag() string {
	return r.Tag
}

func (r BaseRecord) GetRawValue() string {
	return r.RawValue
}

func (r BaseRecord) GetRawLog() string {
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

	return BaseRecord{Tag: tag, RawValue: value, rawLog: rawLog}, nil
}

// BeginRecord represents the start of a transaction log
type BeginRecord struct {
	BaseRecord
	RecordType string // sess, req, bereq, ...
	Parent     VXID   // parent ID
	ESILevel   int    // ESI level, 0 if not an ESI
	Reason     string // reason of the transaction
}

func NewBeginRecord(blr BaseRecord) (BeginRecord, error) {
	parts := strings.Fields(blr.GetRawValue())
	if len(parts) != 3 && len(parts) != 4 {
		return BeginRecord{}, fmt.Errorf("conversion to BeginRecord failed, incorrect len on line %q", blr.GetRawLog())
	}
	if len(parts) == 4 {
		if parts[2] != "esi" {
			return BeginRecord{}, fmt.Errorf("conversion to BeginRecord failed, len is 4 but it is not an ESI on line %q", blr.GetRawLog())
		}
		level, err := strconv.Atoi(parts[3])
		if err != nil {
			return BeginRecord{}, fmt.Errorf("conversion to BeginRecord failed, extraction of ESI level failed on line %q, error: %s", blr.GetRawLog(), err)
		}
		parentVXID, err := parseVXID(parts[1])
		if err != nil {
			return BeginRecord{}, fmt.Errorf("conversion to BeginRecord failed, bad VXID on line %q, error: %s", blr.GetRawLog(), err)
		}

		return BeginRecord{BaseRecord: blr, RecordType: parts[0], Parent: parentVXID, ESILevel: level, Reason: parts[2]}, nil
	}

	parentVXID, err := parseVXID(parts[1])
	if err != nil {
		return BeginRecord{}, fmt.Errorf("conversion to BeginRecord failed, bad VXID on line %q, error: %s", blr.GetRawLog(), err)
	}

	return BeginRecord{BaseRecord: blr, RecordType: parts[0], Parent: parentVXID, ESILevel: 0, Reason: parts[2]}, nil
}

// HeaderRecord represents an HTTP header log record
type HeaderRecord struct {
	BaseRecord
	Name       string // Name of the header
	Value      string // Value of the header
	HeaderType string // Type (ReqHeader, BereqHeader, ...)
}

func (r HeaderRecord) IsRespHeader() bool {
	switch r.HeaderType {
	case tags.RespHeader, tags.BerespHeader:
		return true
	}
	return false
}

// IsRespHeader returns true if its a response header
func NewHeaderRecord(blr BaseRecord) (HeaderRecord, error) {
	fields := strings.SplitAfterN(blr.GetRawValue(), ":", 2)
	if len(fields) < 2 {
		return HeaderRecord{}, fmt.Errorf("conversion to HeaderRecord failed on line %q", blr.GetRawLog())
	}

	name := fields[0]
	firstIndex := strings.Index(blr.GetRawValue(), name)
	value := strings.TrimLeft(blr.GetRawValue()[firstIndex+len(name):], " \t")

	name = strings.TrimRight(name, ": \t")
	// Canonical format for the header key
	name = CanonicalHeaderName(name)

	var hdrType string
	switch blr.GetTag() {
	case tags.ReqHeader:
		hdrType = tags.ReqHeader
	case tags.RespHeader:
		hdrType = tags.RespHeader
	case tags.BereqHeader:
		hdrType = tags.BereqHeader
	case tags.BerespHeader:
		hdrType = tags.BerespHeader
	case tags.ObjHeader:
		hdrType = tags.ObjHeader
	default:
		return HeaderRecord{}, fmt.Errorf("conversion to HeaderRecord failed (unknown header tag) on line %q", blr.GetRawLog())
	}

	return HeaderRecord{
		BaseRecord: blr,
		Name:       name,
		Value:      value,
		HeaderType: hdrType,
	}, nil
}

// HeaderUnsetRecord represents an HTTP header log record which is being unset
type HeaderUnsetRecord struct {
	BaseRecord
	Name       string // Name of the header
	Value      string // Value of the header
	HeaderType string // Type (ReqUnset, BereqUnset, ...)
}

// IsRespHeader returns true if its a response header
func (r HeaderUnsetRecord) IsRespHeader() bool {
	switch r.HeaderType {
	case tags.RespUnset, tags.BerespUnset:
		return true
	}
	return false
}

func NewHeaderUnsetRecord(blr BaseRecord) (HeaderUnsetRecord, error) {
	fields := strings.SplitAfterN(blr.GetRawValue(), ":", 2)
	if len(fields) < 2 {
		return HeaderUnsetRecord{}, fmt.Errorf("conversion to HeaderUnsetRecord failed on line %q", blr.GetRawLog())
	}

	name := fields[0]
	firstIndex := strings.Index(blr.GetRawValue(), name)
	value := strings.TrimLeft(blr.GetRawValue()[firstIndex+len(name):], " \t")

	name = strings.TrimRight(name, ": \t")
	// Canonical format for the header key
	name = CanonicalHeaderName(name)

	var hdrType string
	switch blr.GetTag() {
	case tags.ReqUnset:
		hdrType = tags.ReqUnset
	case tags.RespUnset:
		hdrType = tags.RespUnset
	case tags.BereqUnset:
		hdrType = tags.BereqUnset
	case tags.BerespUnset:
		hdrType = tags.BerespUnset
	case tags.ObjUnset:
		hdrType = tags.ObjUnset
	default:
		return HeaderUnsetRecord{}, fmt.Errorf("conversion to HeaderUnsetRecord failed (unknown header tag) on line %q", blr.GetRawLog())
	}

	return HeaderUnsetRecord{
		BaseRecord: blr,
		Name:       name,
		Value:      value,
		HeaderType: hdrType,
	}, nil
}

// BackendOpenRecord holds information about a new backend connection
type BackendOpenRecord struct {
	BaseRecord
	FileDescriptor int    // Connection file descriptor
	Name           string // Backend display name
	RemoteAddr     net.IP // Remote addr connecting
	RemotePort     int    // Remote port
	LocalAddr      net.IP // Local addr
	LocalPort      int    // Local port
	Reason         string // connect or reuse
}

func (r BackendOpenRecord) String() string {
	return fmt.Sprintf("%s (%s:%d) %s", r.Name, r.RemoteAddr.String(), r.RemotePort, r.Reason)
}

func NewBackendOpenRecord(blr BaseRecord) (BackendOpenRecord, error) {
	parts := strings.Fields(blr.GetRawValue())
	if len(parts) < 6 {
		return BackendOpenRecord{}, fmt.Errorf("conversion to BackendOpenRecord failed, incorrect len on line %q", blr.GetRawLog())
	}

	fileDesc, err := strconv.Atoi(parts[0])
	if err != nil {
		return BackendOpenRecord{}, fmt.Errorf("conversion to BackendOpenRecord failed, bad file descriptor on line %q", blr.GetRawLog())
	}

	remoteAddr := net.ParseIP(parts[2])
	if remoteAddr == nil {
		return BackendOpenRecord{}, fmt.Errorf("conversion to BackendOpenRecord failed, bad remoteAddr on line %q", blr.GetRawLog())
	}

	remotePort, err := strconv.Atoi(parts[3])
	if err != nil {
		return BackendOpenRecord{}, fmt.Errorf("conversion to BackendOpenRecord failed, bad remotePort on line %q", blr.GetRawLog())
	}

	localAddr := net.ParseIP(parts[4])
	if localAddr == nil {
		return BackendOpenRecord{}, fmt.Errorf("conversion to BackendOpenRecord failed, bad localAddr on line %q", blr.GetRawLog())
	}

	localPort, err := strconv.Atoi(parts[5])
	if err != nil {
		return BackendOpenRecord{}, fmt.Errorf("conversion to BackendOpenRecord failed, bad localPort on line %q", blr.GetRawLog())
	}

	reason := "-"
	if len(parts) >= 7 {
		reason = parts[6]
	}

	return BackendOpenRecord{
		BaseRecord:     blr,
		FileDescriptor: fileDesc,
		Name:           parts[1],
		RemoteAddr:     remoteAddr,
		RemotePort:     remotePort,
		LocalAddr:      localAddr,
		LocalPort:      localPort,
		Reason:         reason,
	}, nil
}

// BackendStartRecord holds information about a new backend connection
type BackendStartRecord struct {
	BaseRecord
	RemoteAddr net.IP // Remote address
	RemotePort int    // Remote port
}

func NewBackendStartRecord(blr BaseRecord) (BackendStartRecord, error) {
	parts := strings.Fields(blr.GetRawValue())
	if len(parts) < 2 {
		return BackendStartRecord{}, fmt.Errorf("conversion to BackendStartRecord failed, incorrect len on line %q", blr.GetRawLog())
	}

	remoteAddr := net.ParseIP(parts[0])
	if remoteAddr == nil {
		return BackendStartRecord{}, fmt.Errorf("conversion to BackendStartRecord failed, bad remoteAddr on line %q", blr.GetRawLog())
	}

	remotePort, err := strconv.Atoi(parts[1])
	if err != nil {
		return BackendStartRecord{}, fmt.Errorf("conversion to BackendStartRecord failed, bad remotePort on line %q", blr.GetRawLog())
	}

	return BackendStartRecord{
		BaseRecord: blr,
		RemoteAddr: remoteAddr,
		RemotePort: remotePort,
	}, nil
}

// BackendCloseRecord holds information about a backend connection close
type BackendCloseRecord struct {
	BaseRecord
	FileDescriptor int    // Connection file descriptor
	Name           string // Backend display name
	Reason         string // "close" or "recycle"
}

func NewBackendCloseRecord(blr BaseRecord) (BackendCloseRecord, error) {
	parts := strings.Fields(blr.GetRawValue())
	if len(parts) < 2 {
		return BackendCloseRecord{}, fmt.Errorf("conversion to BackendCloseRecord failed, incorrect len on line %q", blr.GetRawLog())
	}

	fileDesc, err := strconv.Atoi(parts[0])
	if err != nil {
		return BackendCloseRecord{}, fmt.Errorf("conversion to BackendCloseRecord failed, bad file descriptor on line %q", blr.GetRawLog())
	}

	reason := "unknown"
	if len(parts) >= 3 {
		reason = parts[2]
	}

	return BackendCloseRecord{
		BaseRecord:     blr,
		FileDescriptor: fileDesc,
		Name:           parts[1],
		Reason:         reason,
	}, nil
}

// AcctRecord holds accounting information for ReqAcct and BereqAcct tags
type AcctRecord struct {
	BaseRecord
	HeaderTx SizeValue // Header bytes transmitted
	BodyTx   SizeValue // Body bytes transmitted
	TotalTx  SizeValue // Total bytes transmitted
	HeaderRx SizeValue // Header bytes received
	BodyRx   SizeValue // Body bytes received
	TotalRx  SizeValue // Total bytes received
}

func (r AcctRecord) String() string {
	return fmt.Sprintf(
		"Tx(hdr %s, body %s, total %s) | Rx(hdr %s, body %s, total %s)",
		r.HeaderTx.String(),
		r.BodyTx.String(),
		r.TotalTx.String(),
		r.HeaderRx.String(),
		r.BodyRx.String(),
		r.TotalRx.String(),
	)
}

func NewAcctRecord(blr BaseRecord) (AcctRecord, error) {
	parts := strings.Fields(blr.GetRawValue())
	if len(parts) != 6 {
		return AcctRecord{}, fmt.Errorf("conversion to AcctRecord failed, incorrect len on line %q", blr.GetRawLog())
	}

	record := AcctRecord{BaseRecord: blr}
	fields := []*SizeValue{
		&record.HeaderTx,
		&record.BodyTx,
		&record.TotalTx,
		&record.HeaderRx,
		&record.BodyRx,
		&record.TotalRx,
	}

	for i := range fields {
		value, err := strconv.Atoi(parts[i])
		if err != nil {
			return AcctRecord{}, fmt.Errorf("conversion to AcctRecord failed, bad value in part[%d] on line %q", i, blr.GetRawLog())
		}
		*fields[i] = SizeValue(value)
	}

	return record, nil
}

type TimestampRecord struct {
	BaseRecord
	EventLabel   string        // Start, Req, Fetch, Process, Resp, ...
	StartTime    time.Time     // Start time of the timestamp (absoluteTime - sinceLast)
	AbsoluteTime time.Time     // Absolute time of the timestamp (end time, when the record was logged)
	SinceStart   time.Duration // Duration since the start of the tx
	SinceLast    time.Duration // Duration since the last timestamp
}

// String returns the timestamp in a human readable string
func (r TimestampRecord) String() string {
	return fmt.Sprintf(
		"%s | Elapsed: %s | Total: %s",
		r.EventLabel, r.SinceLast.String(), r.SinceStart.String(),
	)
}

func NewTimestampRecord(blr BaseRecord) (TimestampRecord, error) {
	parts := strings.Fields(blr.GetRawValue())
	if len(parts) != 4 {
		return TimestampRecord{}, fmt.Errorf("conversion to TimestampRecord failed, incorrect len on line %q", blr.GetRawLog())
	}

	ab, err := convertToUnixTimestamp(parts[1])
	if err != nil {
		return TimestampRecord{}, fmt.Errorf("conversion to TimestampRecord failed, bad field absolute time on line %q", blr.GetRawLog())
	}

	sinceStart, err := convertStrToDuration(parts[2], time.Second)
	if err != nil {
		return TimestampRecord{}, fmt.Errorf("conversion to TimestampRecord failed, bad field since start on line %q", blr.GetRawLog())
	}
	sinceLast, err := convertStrToDuration(parts[3], time.Second)
	if err != nil {
		return TimestampRecord{}, fmt.Errorf("conversion to TimestampRecord failed, bad field since last on line %q", blr.GetRawLog())
	}

	return TimestampRecord{
		BaseRecord:   blr,
		EventLabel:   strings.TrimRight(parts[0], ":"),
		StartTime:    ab.Add(-sinceLast),
		AbsoluteTime: ab,
		SinceStart:   sinceStart,
		SinceLast:    sinceLast,
	}, nil
}

// ReqStartRecord holds information about the start of request processing
type ReqStartRecord struct {
	BaseRecord
	ClientIP   net.IP // Client IP4/6 address (0.0.0.0 for UDS)
	ClientPort int    // Client Port number (0 for Unix domain sockets)
	Listener   string // Listener name (from -a)
}

func NewReqStartRecord(blr BaseRecord) (ReqStartRecord, error) {
	parts := strings.Fields(blr.GetRawValue())
	if len(parts) != 3 {
		return ReqStartRecord{}, fmt.Errorf("conversion to ReqStartRecord failed, incorrect len on line %q", blr.GetRawLog())
	}

	clientIP := net.ParseIP(parts[0])
	if clientIP == nil {
		return ReqStartRecord{}, fmt.Errorf("conversion to BackendOpenRecord failed, bad clientAddr on line %q", blr.GetRawLog())
	}

	clientPort, err := strconv.Atoi(parts[1])
	if err != nil {
		return ReqStartRecord{}, fmt.Errorf("conversion to BackendOpenRecord failed, bad clientPort on line %q", blr.GetRawLog())
	}

	return ReqStartRecord{BaseRecord: blr, ClientIP: clientIP, ClientPort: clientPort, Listener: parts[2]}, nil
}

// LinkRecord Links to a child transaction
type LinkRecord struct {
	BaseRecord
	TXID     TXID   // Custom transaction ID
	VXID     VXID   // Child vxid
	TXType   string // Child type ("sess", "req" or "bereq")
	Reason   string // Reason
	ESILevel int    // Child task sub-level
}

func NewLinkRecord(blr BaseRecord) (LinkRecord, error) {
	parts := strings.Fields(blr.GetRawValue())
	if len(parts) != 3 && len(parts) != 4 {
		return LinkRecord{}, fmt.Errorf("conversion to LinkRecord failed, incorrect len on line %q", blr.GetRawLog())
	}
	if len(parts) == 4 {
		if parts[2] != "esi" {
			return LinkRecord{}, fmt.Errorf("conversion to LinkRecord failed, len is 4 but it is not an ESI on line %q", blr.GetRawLog())
		}
		level, err := strconv.Atoi(parts[3])
		if err != nil {
			return LinkRecord{}, fmt.Errorf("conversion to LinkRecord failed, extraction of ESI level failed on line %q, error: %s", blr.GetRawLog(), err)
		}
		vxid, err := parseVXID(parts[1])
		if err != nil {
			return LinkRecord{}, fmt.Errorf("conversion to LinkRecord failed, bad VXID on line %q, error: %s", blr.GetRawLog(), err)
		}

		return LinkRecord{
			BaseRecord: blr,
			TXID:       parseTXID(vxid, parts[0], parts[2], level),
			TXType:     parts[0],
			VXID:       vxid,
			ESILevel:   level,
			Reason:     parts[2],
		}, nil
	}

	vxid, err := parseVXID(parts[1])
	if err != nil {
		return LinkRecord{}, fmt.Errorf("conversion to LinkRecord failed, bad VXID on line %q, error: %s", blr.GetRawLog(), err)
	}

	return LinkRecord{
		BaseRecord: blr,
		TXID:       parseTXID(vxid, parts[0], parts[2], 0),
		TXType:     parts[0],
		VXID:       vxid,
		ESILevel:   0,
		Reason:     parts[2],
	}, nil
}

// URLRecord holds request URL from ReqURL and BereqURL tags
type URLRecord struct {
	BaseRecord
	URL url.URL // Request URL
}

func (u URLRecord) MarshalJSON() ([]byte, error) {
	aux := struct {
		BaseRecord
		Path        string
		QueryString string
	}{
		BaseRecord:  u.BaseRecord,
		Path:        u.Path(),
		QueryString: u.QueryString(),
	}
	return json.Marshal(aux)
}

func (r URLRecord) Path() string {
	return r.URL.Path
}

func (r URLRecord) QueryString() string {
	return r.URL.Query().Encode()
}

func NewURLRecord(blr BaseRecord) (URLRecord, error) {
	url, err := url.Parse(blr.GetRawValue())
	if err != nil {
		return URLRecord{}, fmt.Errorf("conversion to URLRecord failed, could not parse URL on line %q", blr.GetRawLog())
	}

	return URLRecord{BaseRecord: blr, URL: *url}, nil
}

// FiltersRecord holds the list of filters applied to the body
type FiltersRecord struct {
	BaseRecord
	Filters []string // List of filters applied to the body
}

func NewFiltersRecord(blr BaseRecord) (FiltersRecord, error) {
	return FiltersRecord{BaseRecord: blr, Filters: strings.Fields(blr.GetRawValue())}, nil
}

// StatusRecord represents an HTTP code response status
type StatusRecord struct {
	BaseRecord
	Status int // HTTP Status code
}

func NewStatusRecord(blr BaseRecord) (StatusRecord, error) {
	v, err := strconv.Atoi(blr.GetRawValue())
	if err != nil {
		return StatusRecord{}, fmt.Errorf("conversion to StatusRecord failed, bad field status on line %q", blr.GetRawLog())
	}
	return StatusRecord{BaseRecord: blr, Status: v}, nil
}

// LengthRecord represents the size of a fetch body
type LengthRecord struct {
	BaseRecord
	Size SizeValue // Size of the fetch body
}

func NewLengthRecord(blr BaseRecord) (LengthRecord, error) {
	size, err := strconv.Atoi(blr.GetRawValue())
	if err != nil {
		return LengthRecord{}, fmt.Errorf("conversion to LengthRecord failed, bad size value on line %q", blr.GetRawLog())
	}
	return LengthRecord{BaseRecord: blr, Size: SizeValue(size)}, nil
}

// HitRecord contains information about a hit of an object in the cache
type HitRecord struct {
	BaseRecord
	ObjVXID VXID          // object VXID
	TTL     time.Duration // remaining TTL
	Grace   time.Duration // grace period
	Keep    time.Duration // keep period
}

func (r HitRecord) String() string {
	return fmt.Sprintf(
		"%d | TTL: %s | Grace: %s | Keep: %s",
		r.ObjVXID,
		r.TTL.String(),
		r.Grace.String(),
		r.Keep.String(),
	)
}

func NewHitRecord(blr BaseRecord) (HitRecord, error) {
	parts := strings.Fields(blr.GetRawValue())
	if len(parts) < 4 {
		return HitRecord{}, fmt.Errorf("conversion to HitRecord failed, incorrect len on line %q", blr.GetRawLog())
	}

	vxid, err := parseVXID(parts[0])
	if err != nil {
		return HitRecord{}, fmt.Errorf("conversion to HitRecord failed, bad VXID on line %q", blr.GetRawLog())
	}

	ttl, err := convertStrToDuration(parts[1], time.Second)
	if err != nil {
		return HitRecord{}, fmt.Errorf("conversion to HitRecord failed, bad field TTL on line %q", blr.GetRawLog())
	}

	grace, err := convertStrToDuration(parts[2], time.Second)
	if err != nil {
		return HitRecord{}, fmt.Errorf("conversion to HitRecord failed, bad field grace on line %q", blr.GetRawLog())
	}

	keep, err := convertStrToDuration(parts[3], time.Second)
	if err != nil {
		return HitRecord{}, fmt.Errorf("conversion to HitRecord failed, bad field keep on line %q", blr.GetRawLog())
	}

	return HitRecord{BaseRecord: blr, ObjVXID: vxid, TTL: ttl, Grace: grace, Keep: keep}, nil
}

// HitMissRecord contains information about a hit for miss object in cache.
type HitMissRecord struct {
	BaseRecord
	ObjVXID VXID          // object VXID
	TTL     time.Duration // remaining TTL
}

func (r HitMissRecord) String() string {
	return fmt.Sprintf(
		"%d | TTL: %s",
		r.ObjVXID,
		r.TTL.String(),
	)
}

func NewHitMissRecord(blr BaseRecord) (HitMissRecord, error) {
	parts := strings.Fields(blr.GetRawValue())
	if len(parts) < 2 {
		return HitMissRecord{}, fmt.Errorf("conversion to HitMissRecord failed, incorrect len on line %q", blr.GetRawLog())
	}

	vxid, err := parseVXID(parts[0])
	if err != nil {
		return HitMissRecord{}, fmt.Errorf("conversion to HitMissRecord failed, bad VXID on line %q", blr.GetRawLog())
	}

	ttl, err := convertStrToDuration(parts[1], time.Second)
	if err != nil {
		return HitMissRecord{}, fmt.Errorf("conversion to HitMissRecord failed, bad field TTL on line %q", blr.GetRawLog())
	}

	return HitMissRecord{BaseRecord: blr, ObjVXID: vxid, TTL: ttl}, nil
}

// TTLRecord reprensets the ttl, grace, keep values for an object
type TTLRecord struct {
	BaseRecord
	Source      string        // "RFC", "VCL" or "HFP"
	TTL         time.Duration // Time-to-live
	Grace       time.Duration // Grace period
	Keep        time.Duration // Keep period
	Reference   time.Time     // Reference time for TTL
	Age         time.Time     // Age (incl Age: header value)
	Date        time.Time     // Date header
	Expires     time.Time     // Expires header
	MaxAge      time.Duration // Max-Age from Cache-Control header
	CacheStatus string        // "cacheable" or "uncacheable"
}

func (r TTLRecord) String() string {
	if r.Source == "RFC" {
		return fmt.Sprintf(
			"%s | TTL %s, Grace %s, Keep %s, Reference %d, Age %d, Date %d, Expires %d, Max-Age %s | %s",
			r.Source,
			r.TTL.String(),
			r.Grace.String(),
			r.Keep.String(),
			r.Reference.Unix(),
			r.Age.Unix(),
			r.Date.Unix(),
			r.Expires.Unix(),
			r.MaxAge.String(),
			r.CacheStatus,
		)
	}

	return fmt.Sprintf(
		"%s | TTL %s, Grace %s, Keep %s, Reference %d | %s",
		r.Source,
		r.TTL.String(),
		r.Grace.String(),
		r.Keep.String(),
		r.Reference.Unix(),
		r.CacheStatus,
	)
}

func NewTTLRecord(blr BaseRecord) (TTLRecord, error) {
	// RFC 120 10 0 1606398419 1606398419 1606398419 0 0 cacheable
	// VCL 120 10 0 1606400537 uncacheable
	// HFP 10 0 0 1606402666 uncacheable
	parts := strings.Fields(blr.GetRawValue())
	if len(parts) < 6 {
		return TTLRecord{}, fmt.Errorf("conversion to TTLRecord failed, incorrect len on line %q", blr.GetRawLog())
	}

	r := TTLRecord{BaseRecord: blr, Source: parts[0]}

	// First 5 parts are common
	ttl, err := strconv.Atoi(parts[1])
	if err != nil {
		return r, fmt.Errorf("conversion to TTLRecord failed, bad field ttl on line %q", blr.GetRawLog())
	}
	r.TTL = time.Duration(ttl * int(time.Second))

	grace, err := strconv.Atoi(parts[2])
	if err != nil {
		return r, fmt.Errorf("conversion to TTLRecord failed, bad field grace on line %q", blr.GetRawLog())
	}
	r.Grace = time.Duration(grace * int(time.Second))

	keep, err := strconv.Atoi(parts[3])
	if err != nil {
		return r, fmt.Errorf("conversion to TTLRecord failed, bad field keep on line %q", blr.GetRawLog())
	}
	r.Keep = time.Duration(keep * int(time.Second))

	ref, err := convertToUnixTimestamp(parts[4])
	if err != nil {
		return r, fmt.Errorf("conversion to TTLRecord failed, bad field reference on line %q", blr.GetRawLog())
	}
	r.Reference = ref

	// Check if we are parsing a VCL or HFP source (6 fields) or a HFP
	if len(parts) == 6 {
		r.CacheStatus = parts[5]
		return r, nil
	}

	if len(parts) != 10 {
		return TTLRecord{}, fmt.Errorf("conversion to TTLRecord failed, incorrect len (wanted 10) on line %q", blr.GetRawLog())
	}

	age, err := convertToUnixTimestamp(parts[5])
	if err != nil {
		return r, fmt.Errorf("conversion to TTLRecord failed, bad field age on line %q", blr.GetRawLog())
	}
	r.Age = age

	date, err := convertToUnixTimestamp(parts[6])
	if err != nil {
		return r, fmt.Errorf("conversion to TTLRecord failed, bad field date on line %q", blr.GetRawLog())
	}
	r.Date = date

	expires, err := convertToUnixTimestamp(parts[7])
	if err != nil {
		return r, fmt.Errorf("conversion to TTLRecord failed, bad field expires on line %q", blr.GetRawLog())
	}
	r.Expires = expires

	maxAge, err := strconv.Atoi(parts[8])
	if err != nil {
		return r, fmt.Errorf("conversion to TTLRecord failed, bad field maxAge on line %q", blr.GetRawLog())
	}
	r.MaxAge = time.Duration(maxAge * int(time.Second))

	// Last field
	r.CacheStatus = parts[9]

	return r, nil
}

// VCLLogRecord holds vsl tag VCL_Log
// key is empty if the log value is not formatted as 'key: value'
type VCLLogRecord struct {
	BaseRecord
	Key   string // Only if the format of the log is 'Key: value'
	Value string
}

func (r VCLLogRecord) String() string {
	if r.Key != "" {
		return r.Key + ": " + r.Value
	}
	return r.Value
}

func NewVCLLogRecord(blr BaseRecord) (VCLLogRecord, error) {
	fields := strings.SplitAfterN(blr.GetRawValue(), ":", 2)
	if len(fields) < 2 {
		return VCLLogRecord{BaseRecord: blr, Key: "", Value: blr.GetRawValue()}, nil
	}

	key := fields[0]
	firstIndex := strings.Index(blr.GetRawValue(), key)
	value := strings.TrimLeft(blr.GetRawValue()[firstIndex+len(key):], " \t")
	return VCLLogRecord{BaseRecord: blr, Key: strings.TrimRight(key, ": \t"), Value: value}, nil
}

// StorageRecord holds the type and name of the storage backend the object is stored in
type StorageRecord struct {
	BaseRecord
	StorageType string // Type ("malloc", "file", "persistent" etc.)
	Name        string // Name of storage backend
}

func NewStorageRecord(blr BaseRecord) (StorageRecord, error) {
	parts := strings.Fields(blr.GetRawValue())
	if len(parts) < 2 {
		return StorageRecord{}, fmt.Errorf("conversion to StorageRecord failed, incorrect len on line %q", blr.GetRawLog())
	}

	return StorageRecord{BaseRecord: blr, StorageType: parts[0], Name: parts[1]}, nil
}

// FetchBodyRecord holds information about the mode to fetch the object from the backend
type FetchBodyRecord struct {
	BaseRecord
	Mode        int    // Body fetch mode
	Description string // Description of body fetch mode
	Stream      bool   // Whether it is a stream fetch
}

func NewFetchBodyRecord(blr BaseRecord) (FetchBodyRecord, error) {
	parts := strings.Fields(blr.GetRawValue())
	if len(parts) != 3 {
		return FetchBodyRecord{}, fmt.Errorf("conversion to FetchBodyRecord failed, incorrect len on line %q", blr.GetRawLog())
	}

	m, err := strconv.Atoi(parts[0])
	if err != nil {
		return FetchBodyRecord{}, fmt.Errorf("conversion to FetchBodyRecord failed, bad field mode on line %q", blr.GetRawLog())
	}

	stream := false
	if parts[2] == "stream" {
		stream = true
	} else if parts[2] != "-" {
		return FetchBodyRecord{}, fmt.Errorf("conversion to FetchBodyRecord failed, unknown value for stream on line %q", blr.GetRawLog())
	}

	return FetchBodyRecord{BaseRecord: blr, Mode: m, Description: parts[1], Stream: stream}, nil
}

// SessOpenRecord is the first record for a client connection
// with the socket-endpoints of the connection
type SessOpenRecord struct {
	BaseRecord
	RemoteAddr     net.IP    // Remote IPv4/6 address / 0.0.0.0 for UDS
	RemotePort     int       // Remote TCP port / 0 for UDS
	SocketName     string    // Socket name (from -a argument)
	LocalAddr      net.IP    // Local IPv4/6 address / 0.0.0.0 for UDS
	LocalPort      int       // Local TCP port / 0 for UDS
	SessionStart   time.Time // Session start time (unix epoch)
	FileDescriptor int       // File descriptor number
}

func (r SessOpenRecord) String() string {
	return fmt.Sprintf(
		"%s:%d %s %s:%d (%s) %d",
		r.RemoteAddr,
		r.RemotePort,
		r.SocketName,
		r.LocalAddr,
		r.LocalPort,
		r.SessionStart.UTC(),
		r.FileDescriptor,
	)
}

func NewSessOpenRecord(blr BaseRecord) (SessOpenRecord, error) {
	parts := strings.Fields(blr.GetRawValue())
	if len(parts) != 7 {
		return SessOpenRecord{}, fmt.Errorf("conversion to SessOpenRecord failed, incorrect len on line %q", blr.GetRawLog())
	}

	remoteAddr := net.ParseIP(parts[0])
	if remoteAddr == nil {
		return SessOpenRecord{}, fmt.Errorf("conversion to SessOpenRecord failed, bad remoteAddr on line %q", blr.GetRawLog())
	}

	remotePort, err := strconv.Atoi(parts[1])
	if err != nil {
		return SessOpenRecord{}, fmt.Errorf("conversion to SessOpenRecord failed, bad remotePort on line %q", blr.GetRawLog())
	}

	localAddr := net.ParseIP(parts[3])
	if localAddr == nil {
		return SessOpenRecord{}, fmt.Errorf("conversion to SessOpenRecord failed, bad localAddr on line %q", blr.GetRawLog())
	}

	localPort, err := strconv.Atoi(parts[4])
	if err != nil {
		return SessOpenRecord{}, fmt.Errorf("conversion to SessOpenRecord failed, bad localPort on line %q", blr.GetRawLog())
	}

	sessionStart, err := convertToUnixTimestamp(parts[5])
	if err != nil {
		return SessOpenRecord{}, fmt.Errorf("conversion to SessOpenRecord failed, bad field sessionStart on line %q", blr.GetRawLog())
	}

	fileDesc, err := strconv.Atoi(parts[6])
	if err != nil {
		return SessOpenRecord{}, fmt.Errorf("conversion to SessOpenRecord failed, bad file descriptor on line %q", blr.GetRawLog())
	}

	return SessOpenRecord{
		BaseRecord:     blr,
		RemoteAddr:     remoteAddr,
		RemotePort:     remotePort,
		SocketName:     parts[2],
		LocalAddr:      localAddr,
		LocalPort:      localPort,
		SessionStart:   sessionStart,
		FileDescriptor: fileDesc,
	}, nil
}

// SessCloseRecord is the last record for any client connection
type SessCloseRecord struct {
	BaseRecord
	Reason   string        // Why the connection closed
	Duration time.Duration // How long the session was open
}

func (r SessCloseRecord) String() string {
	return r.Reason + " " + r.Duration.String()
}

func NewSessCloseRecord(blr BaseRecord) (SessCloseRecord, error) {
	parts := strings.Fields(blr.GetRawValue())
	if len(parts) != 2 {
		return SessCloseRecord{}, fmt.Errorf("conversion to SessCloseRecord failed, invalid len on line %q", blr.GetRawLog())
	}
	d, err := convertStrToDuration(parts[1], time.Second)
	if err != nil {
		return SessCloseRecord{}, fmt.Errorf("conversion to SessCloseRecord failed, bad field duration on line %q", blr.GetRawLog())
	}
	return SessCloseRecord{BaseRecord: blr, Reason: parts[0], Duration: d}, nil
}

// GzipRecord holds G(un)zip performed on object
type GzipRecord struct {
	BaseRecord
	Action                    string    // G: Gzip, U: Gunzip, u: Gunzip-test
	When                      string    // F: Fetch, D: Deliver
	Object                    string    // E: ESI, -: Plain object
	InputBytes                SizeValue // Bytes input
	OutputBytes               SizeValue // Bytes output
	BitLocFirst               int64     // Bit location of first deflate block
	BitLocLast                int64     // Bit location of 'last' bit
	BitLengthOfCompressedData int64     // Bit length of compressed data
}

func (r GzipRecord) String() string {
	action := r.Action
	switch r.Action {
	case "G":
		action = "Gzip"
	case "U":
		action = "Gunzip"
	case "u":
		action = "Gunzip-test"
	}

	when := r.When
	switch r.When {
	case "F":
		when = "Fetch"
	case "D":
		when = "Deliver"
	}

	object := r.Object
	switch r.Object {
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
		r.InputBytes.String(),
		r.OutputBytes.String(),
		r.BitLocFirst,
		r.BitLocLast,
		r.BitLengthOfCompressedData,
	)
}

func NewGzipRecord(blr BaseRecord) (GzipRecord, error) {
	parts := strings.Fields(blr.GetRawValue())
	if len(parts) != 8 {
		return GzipRecord{}, fmt.Errorf("conversion to GzipRecord failed, incorrect len on line %q", blr.GetRawLog())
	}

	record := GzipRecord{BaseRecord: blr}

	record.Action = parts[0]
	record.When = parts[1]
	record.Object = parts[2]

	inputBytes, err := strconv.Atoi(parts[3])
	if err != nil {
		return GzipRecord{}, fmt.Errorf("conversion to GzipRecord failed, bad value for inputBytes on line %q", blr.GetRawLog())
	}
	record.InputBytes = SizeValue(inputBytes)

	outputBytes, err := strconv.Atoi(parts[4])
	if err != nil {
		return GzipRecord{}, fmt.Errorf("conversion to GzipRecord failed, bad value for outputBytes on line %q", blr.GetRawLog())
	}
	record.OutputBytes = SizeValue(outputBytes)

	bitLocFirst, err := strconv.ParseInt(parts[5], 10, 64)
	if err != nil {
		return GzipRecord{}, fmt.Errorf("conversion to GzipRecord failed, bad value for bitLocFirst on line %q", blr.GetRawLog())
	}
	record.BitLocFirst = bitLocFirst

	bitLocLast, err := strconv.ParseInt(parts[6], 10, 64)
	if err != nil {
		return GzipRecord{}, fmt.Errorf("conversion to GzipRecord failed, bad value for bitLocLast on line %q", blr.GetRawLog())
	}
	record.BitLocLast = bitLocLast

	bitLengthOfCompressedData, err := strconv.ParseInt(parts[7], 10, 64)
	if err != nil {
		return GzipRecord{}, fmt.Errorf("conversion to GzipRecord failed, bad value for bitLengthOfCompressedData on line %q", blr.GetRawLog())
	}
	record.BitLengthOfCompressedData = bitLengthOfCompressedData

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
