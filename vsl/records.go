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
	// LinkTypeSession is a VSL link from a session.
	LinkTypeSession = "sess"
	// LinkTypeRequest is a VSL link from a request.
	LinkTypeRequest = "req"
	// LinkTypeBereq is a VSL link from a backend request.
	LinkTypeBereq = "bereq"
)

// nolint
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

// Record interface for all the VSL log records.
type Record interface {
	String() string
	GetTag() string
	GetRawValue() string
	GetRawLog() string
}

// BaseRecord is a single VSL log line split by tag and value.
type BaseRecord struct {
	Tag      string `json:"tag"`       // VSL Tag (Begin, Timestamp, ReqURL, ReqHeader, ...)
	RawValue string `json:"raw_value"` // Value after the tag

	rawLog string // Raw log line
}

func NewBaseRecord(rawLog string) (BaseRecord, error) {
	fields := strings.Fields(rawLog)
	if len(fields) < 2 {
		return BaseRecord{}, fmt.Errorf("could not parse line %q", rawLog)
	}

	tag := fields[1] // e.g: Begin
	_, after, _ := strings.Cut(rawLog, tag)
	value := strings.TrimLeft(after, " \t")

	return BaseRecord{Tag: tag, RawValue: value, rawLog: rawLog}, nil
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

// BeginRecord represents the start of a transaction log.
type BeginRecord struct {
	BaseRecord

	RecordType string // sess, req, bereq, ...
	Parent     VXID   // parent ID
	ESILevel   int    // ESI level, 0 if not an ESI
	Reason     string // reason of the transaction
}

func NewBeginRecord(blr BaseRecord) (BeginRecord, error) {
	ref, err := parseTxRef(blr, "BeginRecord")
	if err != nil {
		return BeginRecord{}, err
	}

	return BeginRecord{BaseRecord: blr, RecordType: ref.recordType, Parent: ref.vxid, ESILevel: ref.esiLevel, Reason: ref.reason}, nil
}

// HeaderRecord represents an HTTP header log record.
type HeaderRecord struct {
	BaseRecord

	Name       string // Name of the header
	Value      string // Value of the header
	HeaderType string // Type (ReqHeader, BereqHeader, ...)
}

// NewHeaderRecord creates a new header record.
func NewHeaderRecord(blr BaseRecord) (HeaderRecord, error) {
	fields := strings.SplitAfterN(blr.GetRawValue(), ":", 2)
	if len(fields) < 2 {
		return HeaderRecord{}, fmt.Errorf("conversion to HeaderRecord failed on line %q", blr.GetRawLog())
	}

	name := fields[0]
	value := strings.TrimLeft(blr.GetRawValue()[len(name):], " \t")

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

func (r HeaderRecord) IsRespHeader() bool {
	switch r.HeaderType {
	case tags.RespHeader, tags.BerespHeader:
		return true
	}

	return false
}

// HeaderUnsetRecord represents an HTTP header log record which is being unset.
type HeaderUnsetRecord struct {
	BaseRecord

	Name       string // Name of the header
	Value      string // Value of the header
	HeaderType string // Type (ReqUnset, BereqUnset, ...)
}

func NewHeaderUnsetRecord(blr BaseRecord) (HeaderUnsetRecord, error) {
	fields := strings.SplitAfterN(blr.GetRawValue(), ":", 2)
	if len(fields) < 2 {
		return HeaderUnsetRecord{}, fmt.Errorf("conversion to HeaderUnsetRecord failed on line %q", blr.GetRawLog())
	}

	name := fields[0]
	value := strings.TrimLeft(blr.GetRawValue()[len(name):], " \t")

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

// IsRespHeader returns true if its a response header.
func (r HeaderUnsetRecord) IsRespHeader() bool {
	switch r.HeaderType {
	case tags.RespUnset, tags.BerespUnset:
		return true
	}

	return false
}

// BackendOpenRecord holds information about a new backend connection.
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

func NewBackendOpenRecord(blr BaseRecord) (BackendOpenRecord, error) {
	f := newFieldScanner(blr, "BackendOpenRecord")
	f.requireMin(6)

	record := BackendOpenRecord{
		BaseRecord:     blr,
		FileDescriptor: f.int("file descriptor", 0),
		Name:           f.str("name", 1),
		RemoteAddr:     f.ip("remote address", 2),
		RemotePort:     f.int("remote port", 3),
		LocalAddr:      f.ip("local address", 4),
		LocalPort:      f.int("local port", 5),
		Reason:         f.strOr(6, "-"),
	}

	err := f.err()
	if err != nil {
		return BackendOpenRecord{}, err
	}

	return record, nil
}

func (r BackendOpenRecord) ConnStr() string {
	return net.JoinHostPort(r.RemoteAddr.String(), fmt.Sprintf("%d", r.RemotePort)) // nolint:perfsprint
}

func (r BackendOpenRecord) String() string {
	return fmt.Sprintf("%s (%s) %s", r.Name, r.ConnStr(), r.Reason)
}

// BackendStartRecord holds information about a new backend connection.
type BackendStartRecord struct {
	BaseRecord

	RemoteAddr net.IP // Remote address
	RemotePort int    // Remote port
}

func NewBackendStartRecord(blr BaseRecord) (BackendStartRecord, error) {
	f := newFieldScanner(blr, "BackendStartRecord")
	f.requireMin(2)

	record := BackendStartRecord{
		BaseRecord: blr,
		RemoteAddr: f.ip("remote address", 0),
		RemotePort: f.int("remote port", 1),
	}

	err := f.err()
	if err != nil {
		return BackendStartRecord{}, err
	}

	return record, nil
}

func (r BackendStartRecord) ConnStr() string {
	return net.JoinHostPort(r.RemoteAddr.String(), fmt.Sprintf("%d", r.RemotePort)) // nolint: perfsprint
}

func (r BackendStartRecord) String() string {
	return r.ConnStr()
}

// BackendCloseRecord holds information about a backend connection close.
type BackendCloseRecord struct {
	BaseRecord

	FileDescriptor int    // Connection file descriptor
	Name           string // Backend display name
	Reason         string // "close" or "recycle"
	OptionalReason string // Optional reason
}

func NewBackendCloseRecord(blr BaseRecord) (BackendCloseRecord, error) {
	f := newFieldScanner(blr, "BackendCloseRecord")
	f.requireMin(2)

	record := BackendCloseRecord{
		BaseRecord:     blr,
		FileDescriptor: f.int("file descriptor", 0),
		Name:           f.str("name", 1),
		Reason:         f.strOr(2, ""),
		OptionalReason: f.strOr(3, ""),
	}

	err := f.err()
	if err != nil {
		return BackendCloseRecord{}, err
	}

	return record, nil
}

// BackendReuseRecord holds information about a backend reuse (keep-alive)
//
// Note that this record was deprecated in favor of BackendClose.
type BackendReuseRecord struct {
	BaseRecord

	FileDescriptor int    // Connection file descriptor
	Name           string // Backend display name
}

func NewBackendReuseRecord(blr BaseRecord) (BackendReuseRecord, error) {
	f := newFieldScanner(blr, "BackendReuseRecord")
	if f.count() < 2 {
		return BackendReuseRecord{BaseRecord: blr}, nil
	}

	record := BackendReuseRecord{
		BaseRecord:     blr,
		FileDescriptor: f.int("file descriptor", 0),
		Name:           f.str("name", 1),
	}

	err := f.err()
	if err != nil {
		return BackendReuseRecord{BaseRecord: blr}, err
	}

	return record, nil
}

// AcctRecord holds accounting information for ReqAcct and BereqAcct tags.
//
// The two tags don't share a wire field order: BereqAcct reports what
// Varnish sent to the backend before what it received back (hdr-tx, body-tx, total-tx,
// hdr-rx, body-rx, total-rx), while ReqAcct reports what Varnish received from the client
// before what it sent back (hdr-rx, body-rx, total-rx, hdr-tx, body-tx, total-tx).
type AcctRecord struct {
	BaseRecord

	HeaderTx SizeValue // Header bytes transmitted
	BodyTx   SizeValue // Body bytes transmitted
	TotalTx  SizeValue // Total bytes transmitted
	HeaderRx SizeValue // Header bytes received
	BodyRx   SizeValue // Body bytes received
	TotalRx  SizeValue // Total bytes received
}

func NewAcctRecord(blr BaseRecord) (AcctRecord, error) {
	f := newFieldScanner(blr, "AcctRecord")
	f.require(6)

	record := AcctRecord{BaseRecord: blr}

	if blr.Tag == tags.BereqAcct {
		record.HeaderTx = f.size("header tx", 0)
		record.BodyTx = f.size("body tx", 1)
		record.TotalTx = f.size("total tx", 2)
		record.HeaderRx = f.size("header rx", 3)
		record.BodyRx = f.size("body rx", 4)
		record.TotalRx = f.size("total rx", 5)
	} else {
		record.HeaderRx = f.size("header rx", 0)
		record.BodyRx = f.size("body rx", 1)
		record.TotalRx = f.size("total rx", 2)
		record.HeaderTx = f.size("header tx", 3)
		record.BodyTx = f.size("body tx", 4)
		record.TotalTx = f.size("total tx", 5)
	}

	err := f.err()
	if err != nil {
		return AcctRecord{}, err
	}

	return record, nil
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

// PipeAcctRecord holds accounting information for PipeAcct tags.
type PipeAcctRecord struct {
	BaseRecord

	ClientReqHeaders  SizeValue // Client request headers
	BackendReqHeaders SizeValue // Backend request headers
	PipedFrom         SizeValue // Piped bytes from client
	PipedTo           SizeValue // Piped bytes to client
}

func NewPipeAcctRecord(blr BaseRecord) (PipeAcctRecord, error) {
	f := newFieldScanner(blr, "PipeAcctRecord")
	f.require(4)

	record := PipeAcctRecord{
		BaseRecord:        blr,
		ClientReqHeaders:  f.size("client req headers", 0),
		BackendReqHeaders: f.size("backend req headers", 1),
		PipedFrom:         f.size("piped from", 2),
		PipedTo:           f.size("piped to", 3),
	}

	err := f.err()
	if err != nil {
		return PipeAcctRecord{}, err
	}

	return record, nil
}

func (r PipeAcctRecord) String() string {
	return fmt.Sprintf(
		"Hdr(client %s, backend %s) | Piped(from %s, to %s)",
		r.ClientReqHeaders,
		r.BackendReqHeaders,
		r.PipedFrom,
		r.PipedTo,
	)
}

type TimestampRecord struct {
	BaseRecord

	EventLabel   string        // Start, Req, Fetch, Process, Resp, ...
	StartTime    time.Time     // Start time of the timestamp (absoluteTime - sinceLast)
	AbsoluteTime time.Time     // Absolute time of the timestamp (end time, when the record was logged)
	SinceStart   time.Duration // Duration since the start of the tx
	SinceLast    time.Duration // Duration since the last timestamp
}

func NewTimestampRecord(blr BaseRecord) (TimestampRecord, error) {
	f := newFieldScanner(blr, "TimestampRecord")
	f.require(4)

	label := strings.TrimRight(f.str("event label", 0), ":")
	ab := f.unixTime("absolute time", 1)
	sinceStart := f.duration("since start", 2, time.Second)
	sinceLast := f.duration("since last", 3, time.Second)

	err := f.err()
	if err != nil {
		return TimestampRecord{}, err
	}

	return TimestampRecord{
		BaseRecord:   blr,
		EventLabel:   label,
		StartTime:    ab.Add(-sinceLast),
		AbsoluteTime: ab,
		SinceStart:   sinceStart,
		SinceLast:    sinceLast,
	}, nil
}

// String returns the timestamp in a human readable string.
func (r TimestampRecord) String() string {
	return fmt.Sprintf(
		"%s | Elapsed: %s, Total: %s, %s",
		r.EventLabel, r.SinceLast.String(), r.SinceStart.String(), r.AbsoluteTime.String(),
	)
}

// ReqStartRecord holds information about the start of request processing.
type ReqStartRecord struct {
	BaseRecord

	ClientIP   net.IP // Client IP4/6 address (0.0.0.0 for UDS)
	ClientPort int    // Client Port number (0 for Unix domain sockets)
	Listener   string // Listener name (from -a)
	Scheme     string // Protocol scheme ("http" or "https")
}

func NewReqStartRecord(blr BaseRecord) (ReqStartRecord, error) {
	f := newFieldScanner(blr, "ReqStartRecord")
	f.requireMin(3)

	record := ReqStartRecord{
		BaseRecord: blr,
		ClientIP:   f.ip("client address", 0),
		ClientPort: f.int("client port", 1),
		Listener:   f.str("listener", 2),
		Scheme:     f.strOr(3, ""),
	}

	err := f.err()
	if err != nil {
		return ReqStartRecord{}, err
	}

	return record, nil
}

func (r ReqStartRecord) String() string {
	return r.ConnStr() + " " + r.Listener
}

func (r ReqStartRecord) ConnStr() string {
	return net.JoinHostPort(r.ClientIP.String(), fmt.Sprintf("%d", r.ClientPort)) // nolint:perfsprint
}

// LinkRecord Links to a child transaction.
type LinkRecord struct {
	BaseRecord

	TXID     TXID   // Custom transaction ID
	VXID     VXID   // Child vxid
	TXType   string // Child type ("sess", "req" or "bereq")
	Reason   string // Reason
	ESILevel int    // Child task sub-level
}

func NewLinkRecord(blr BaseRecord) (LinkRecord, error) {
	ref, err := parseTxRef(blr, "LinkRecord")
	if err != nil {
		return LinkRecord{}, err
	}

	return LinkRecord{
		BaseRecord: blr,
		TXID:       parseTXID(ref.vxid, ref.recordType, ref.reason, ref.esiLevel),
		TXType:     ref.recordType,
		VXID:       ref.vxid,
		ESILevel:   ref.esiLevel,
		Reason:     ref.reason,
	}, nil
}

// txRef holds the value shape shared by the Begin and Link tags:
// "<type> <vxid> <reason>" or "<type> <vxid> esi <level>".
type txRef struct {
	recordType string
	vxid       VXID
	reason     string
	esiLevel   int
}

func parseTxRef(blr BaseRecord, target string) (txRef, error) {
	f := newFieldScanner(blr, target)
	f.require(3, 4)

	ref := txRef{
		recordType: f.strOr(0, ""),
		vxid:       f.vxid("vxid", 1),
		reason:     f.strOr(2, ""),
	}

	if f.count() == 4 {
		f.literal("reason", 2, "esi")
		ref.esiLevel = f.int("esi level", 3)
	}

	err := f.err()
	if err != nil {
		return txRef{}, err
	}

	return ref, nil
}

// URLRecord holds request URL from ReqURL and BereqURL tags.
type URLRecord struct {
	BaseRecord

	URL url.URL // Request URL
}

func NewURLRecord(blr BaseRecord) (URLRecord, error) {
	u, err := url.Parse(blr.GetRawValue())
	if err != nil {
		return URLRecord{}, fmt.Errorf("conversion to URLRecord failed, could not parse URL on line %q", blr.GetRawLog())
	}

	return URLRecord{BaseRecord: blr, URL: *u}, nil
}

func (u URLRecord) MarshalJSON() ([]byte, error) {
	aux := struct {
		BaseRecord `json:"base_record"`

		Path        string `json:"path"`
		QueryString string `json:"query_string"`
	}{
		BaseRecord:  u.BaseRecord,
		Path:        u.Path(),
		QueryString: u.QueryString(),
	}

	return json.Marshal(aux)
}

func (u URLRecord) Path() string {
	return u.URL.Path
}

func (u URLRecord) QueryString() string {
	return u.URL.Query().Encode()
}

// FiltersRecord holds the list of filters applied to the body.
type FiltersRecord struct {
	BaseRecord

	Filters []string // List of filters applied to the body
}

func NewFiltersRecord(blr BaseRecord) (FiltersRecord, error) {
	return FiltersRecord{BaseRecord: blr, Filters: strings.Fields(blr.GetRawValue())}, nil
}

// StatusRecord represents an HTTP code response status.
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

// LengthRecord represents the size of a fetch body.
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
//
// It can be either a Hit, HitMiss or HitPass record.
type HitRecord struct {
	BaseRecord

	ObjVXID       VXID          // object VXID
	TTL           time.Duration // remaining TTL
	Grace         time.Duration // grace period
	Keep          time.Duration // keep period
	Fetched       SizeValue     // bytes fetched so far
	ContentLength SizeValue     // Content length
}

func NewHitRecord(blr BaseRecord) (HitRecord, error) {
	f := newFieldScanner(blr, "HitRecord")
	f.require(2, 4, 5, 6)

	hr := HitRecord{
		BaseRecord: blr,
		ObjVXID:    f.vxid("obj vxid", 0),
		TTL:        f.duration("ttl", 1, time.Second),
	}

	if f.count() >= 4 {
		hr.Grace = f.duration("grace", 2, time.Second)
		hr.Keep = f.duration("keep", 3, time.Second)
	}

	if f.count() >= 5 {
		hr.Fetched = f.size("fetched", 4)
	}

	if f.count() == 6 {
		hr.ContentLength = f.size("content length", 5)
	}

	err := f.err()
	if err != nil {
		return HitRecord{}, err
	}

	return hr, nil
}

func (r HitRecord) String() string {
	s := fmt.Sprintf(
		"ObjVXID: %d, TTL: %s",
		r.ObjVXID,
		// Varnish reports these with microsecond precision, which is noise for display purposes.
		r.TTL.Round(time.Second).String(),
	)

	if r.GetTag() == tags.HitMiss || r.GetTag() == tags.HitPass {
		return s
	}

	s += fmt.Sprintf(
		", Grace: %s, Keep: %s",
		r.Grace.Round(time.Second).String(),
		r.Keep.Round(time.Second).String(),
	)

	if r.Fetched != 0 {
		s += fmt.Sprintf(", Fetched: %s", r.Fetched)
	}

	if r.ContentLength != 0 {
		s += fmt.Sprintf(", ContentLength: %s", r.ContentLength)
	}

	return s
}

// TTLRecord reprensets the ttl, grace, keep values for an object.
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

func NewTTLRecord(blr BaseRecord) (TTLRecord, error) {
	// RFC 120 10 0 1606398419 1606398419 1606398419 0 0 cacheable
	// VCL 120 10 0 1606400537 uncacheable
	// HFP 10 0 0 1606402666 uncacheable
	f := newFieldScanner(blr, "TTLRecord")
	f.require(6, 10)

	r := TTLRecord{
		BaseRecord: blr,
		Source:     f.str("source", 0),
		TTL:        f.duration("ttl", 1, time.Second),
		Grace:      f.duration("grace", 2, time.Second),
		Keep:       f.duration("keep", 3, time.Second),
		Reference:  f.unixTime("reference", 4),
	}

	// 6 fields: VCL or HFP source. 10 fields: RFC source.
	if f.count() == 6 {
		r.CacheStatus = f.str("cache status", 5)
	} else {
		r.Age = f.unixTime("age", 5)
		r.Date = f.unixTime("date", 6)
		r.Expires = f.unixTime("expires", 7)
		r.MaxAge = f.duration("max age", 8, time.Second)
		r.CacheStatus = f.str("cache status", 9)
	}

	err := f.err()
	if err != nil {
		return TTLRecord{}, err
	}

	return r, nil
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

// VCLLogRecord holds vsl tag VCL_Log
// key is empty if the log value is not formatted as 'key: value'.
type VCLLogRecord struct {
	BaseRecord

	Key   string // Only if the format of the log is 'Key: value'
	Value string
}

func NewVCLLogRecord(blr BaseRecord) (VCLLogRecord, error) {
	fields := strings.SplitAfterN(blr.GetRawValue(), ":", 2)
	if len(fields) < 2 {
		return VCLLogRecord{BaseRecord: blr, Key: "", Value: blr.GetRawValue()}, nil
	}

	key := fields[0]
	value := strings.TrimLeft(blr.GetRawValue()[len(key):], " \t")

	return VCLLogRecord{BaseRecord: blr, Key: strings.TrimRight(key, ": \t"), Value: value}, nil
}

func (r VCLLogRecord) String() string {
	if r.Key != "" {
		return r.Key + ": " + r.Value
	}

	return r.Value
}

// StorageRecord holds the type and name of the storage backend the object is stored in.
type StorageRecord struct {
	BaseRecord

	StorageType string // Type ("malloc", "file", "persistent" etc.)
	Name        string // Name of storage backend
}

func NewStorageRecord(blr BaseRecord) (StorageRecord, error) {
	f := newFieldScanner(blr, "StorageRecord")
	f.requireMin(2)

	record := StorageRecord{BaseRecord: blr, StorageType: f.str("storage type", 0), Name: f.str("name", 1)}

	err := f.err()
	if err != nil {
		return StorageRecord{}, err
	}

	return record, nil
}

// FetchBodyRecord holds information about the mode to fetch the object from the backend.
type FetchBodyRecord struct {
	BaseRecord

	Mode        int    // Body fetch mode
	Description string // Description of body fetch mode
	Stream      bool   // Whether it is a stream fetch
}

func NewFetchBodyRecord(blr BaseRecord) (FetchBodyRecord, error) {
	f := newFieldScanner(blr, "FetchBodyRecord")
	f.require(3)

	record := FetchBodyRecord{
		BaseRecord:  blr,
		Mode:        f.int("mode", 0),
		Description: f.str("description", 1),
		Stream:      f.oneOf("stream", 2, "stream", "-") == "stream",
	}

	err := f.err()
	if err != nil {
		return FetchBodyRecord{}, err
	}

	return record, nil
}

// SessOpenRecord is the first record for a client connection
// with the socket-endpoints of the connection.
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

func NewSessOpenRecord(blr BaseRecord) (SessOpenRecord, error) {
	f := newFieldScanner(blr, "SessOpenRecord")
	f.require(7)

	record := SessOpenRecord{
		BaseRecord:     blr,
		RemoteAddr:     f.ip("remote address", 0),
		RemotePort:     f.int("remote port", 1),
		SocketName:     f.str("socket name", 2),
		LocalAddr:      f.ip("local address", 3),
		LocalPort:      f.int("local port", 4),
		SessionStart:   f.unixTime("session start", 5),
		FileDescriptor: f.int("file descriptor", 6),
	}

	err := f.err()
	if err != nil {
		return SessOpenRecord{}, err
	}

	return record, nil
}

func (r SessOpenRecord) String() string {
	return fmt.Sprintf(
		"%s %s %s:%d (%s) %d",
		r.ConnStr(),
		r.SocketName,
		r.LocalAddr,
		r.LocalPort,
		r.SessionStart.UTC(),
		r.FileDescriptor,
	)
}

func (r SessOpenRecord) ConnStr() string {
	return net.JoinHostPort(r.RemoteAddr.String(), fmt.Sprintf("%d", r.RemotePort)) // nolint:perfsprint
}

// SessCloseRecord is the last record for any client connection.
type SessCloseRecord struct {
	BaseRecord

	Reason   string        // Why the connection closed
	Duration time.Duration // How long the session was open
}

func NewSessCloseRecord(blr BaseRecord) (SessCloseRecord, error) {
	f := newFieldScanner(blr, "SessCloseRecord")
	f.require(2)

	record := SessCloseRecord{
		BaseRecord: blr,
		Reason:     f.str("reason", 0),
		Duration:   f.duration("duration", 1, time.Second),
	}

	err := f.err()
	if err != nil {
		return SessCloseRecord{}, err
	}

	return record, nil
}

func (r SessCloseRecord) String() string {
	return r.Reason + " " + r.Duration.String()
}

// GzipRecord holds G(un)zip performed on object.
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
	Error                     string    // Parser failure (probably a gzip error)
}

func NewGzipRecord(blr BaseRecord) (GzipRecord, error) {
	f := newFieldScanner(blr, "GzipRecord")
	if f.count() != 8 {
		// It could be a gzip error like: G(un)zip error: -3 ((null))
		return GzipRecord{BaseRecord: blr, Error: blr.GetRawValue()}, nil
	}

	record := GzipRecord{
		BaseRecord:                blr,
		Action:                    f.str("action", 0),
		When:                      f.str("when", 1),
		Object:                    f.str("object", 2),
		InputBytes:                f.size("input bytes", 3),
		OutputBytes:               f.size("output bytes", 4),
		BitLocFirst:               f.int64("bit loc first", 5),
		BitLocLast:                f.int64("bit loc last", 6),
		BitLengthOfCompressedData: f.int64("bit length of compressed data", 7),
	}

	err := f.err()
	if err != nil {
		return GzipRecord{}, err
	}

	return record, nil
}

func (r GzipRecord) String() string {
	if r.Error != "" {
		return r.Error
	}

	var action string

	switch r.Action {
	case "G":
		action = "Gzip"
	case "U":
		action = "Gunzip"
	case "u":
		action = "Gunzip-test"
	default:
		action = "unknown-gzip-action"
	}

	var when string

	switch r.When {
	case "F":
		when = "Fetch"
	case "D":
		when = "Deliver"
	default:
		when = "unknown-gzip-when"
	}

	var object string

	switch r.Object {
	case "E":
		object = "ESI"
	case "-":
		object = "Plain"
	default:
		object = "unknown-gzip-object"
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

// MSE4NewObjectRecord holds MSE4 new object timing data.
type MSE4NewObjectRecord struct {
	BaseRecord

	AllocationChunks     int64         // Number of allocation chunks created
	BytesProcessed       SizeValue     // Number of bytes processed
	TimeElapsed          time.Duration // Time elapsed between object creation and finalization (seconds)
	TimeMSE4Processing   time.Duration // Total time spent on MSE4 processing (seconds)
	TimeMemoryAllocation time.Duration // Time spent allocating memory (seconds)
	TimeResourceWait     time.Duration // Time spent waiting for resource acquisition (seconds) [persisted only]
	TimeDiskIOFetch      time.Duration // Time spent waiting for disk IO during fetch (seconds) [persisted only]
	TimeDiskIOFinalize   time.Duration // Time spent waiting for disk IO during finalization (seconds) [persisted only]
	IsPersisted          bool          // Whether this object was persisted to disk
}

func NewMSE4NewObjectRecord(blr BaseRecord) (MSE4NewObjectRecord, error) {
	f := newFieldScanner(blr, "MSE4NewObjectRecord")
	f.require(5, 8)

	record := MSE4NewObjectRecord{
		BaseRecord:           blr,
		IsPersisted:          f.count() == 8,
		AllocationChunks:     f.int64("allocation chunks", 0),
		BytesProcessed:       f.size("bytes processed", 1),
		TimeElapsed:          f.duration("time elapsed", 2, time.Second),
		TimeMSE4Processing:   f.duration("time mse4 processing", 3, time.Second),
		TimeMemoryAllocation: f.duration("time memory allocation", 4, time.Second),
	}

	// Optional persisted object fields
	if record.IsPersisted {
		record.TimeResourceWait = f.duration("time resource wait", 5, time.Second)
		record.TimeDiskIOFetch = f.duration("time disk io fetch", 6, time.Second)
		record.TimeDiskIOFinalize = f.duration("time disk io finalize", 7, time.Second)
	}

	err := f.err()
	if err != nil {
		return MSE4NewObjectRecord{}, err
	}

	return record, nil
}

func (r MSE4NewObjectRecord) String() string {
	s := fmt.Sprintf(
		"%d chunks | %s processed | %s elapsed | %s MSE4 | %s mem alloc",
		r.AllocationChunks,
		r.BytesProcessed.String(),
		r.TimeElapsed,
		r.TimeMSE4Processing,
		r.TimeMemoryAllocation,
	)

	if r.IsPersisted {
		s += fmt.Sprintf(
			" | %s resource wait | %s disk IO fetch | %s disk IO finalize",
			r.TimeResourceWait,
			r.TimeDiskIOFetch,
			r.TimeDiskIOFinalize,
		)
	}

	return s
}

// MSE4ObjIterRecord holds MSE4 object payload iteration timing data.
type MSE4ObjIterRecord struct {
	BaseRecord

	TimeElapsed          time.Duration // Time elapsed between start and end of iteration (seconds)
	BytesProcessed       SizeValue     // Number of bytes processed
	TimeProcessing       time.Duration // Total time spent processing (seconds)
	TimeBackendWait      time.Duration // Time spent waiting for backend data (seconds)
	DiskIOBytes          SizeValue     // Disk IO bytes processed [persisted only]
	TimeDiskIOProcessing time.Duration // Time spent on processing disk IO (seconds) [persisted only]
	IsPersisted          bool          // Whether this object was persisted to disk
}

func NewMSE4ObjIterRecord(blr BaseRecord) (MSE4ObjIterRecord, error) {
	f := newFieldScanner(blr, "MSE4ObjIterRecord")
	f.require(4, 6)

	record := MSE4ObjIterRecord{
		BaseRecord:      blr,
		IsPersisted:     f.count() == 6,
		TimeElapsed:     f.duration("time elapsed", 0, time.Second),
		BytesProcessed:  f.size("bytes processed", 1),
		TimeProcessing:  f.duration("time processing", 2, time.Second),
		TimeBackendWait: f.duration("time backend wait", 3, time.Second),
	}

	// Optional persisted object fields
	if record.IsPersisted {
		record.DiskIOBytes = f.size("disk io bytes", 4)
		record.TimeDiskIOProcessing = f.duration("time disk io processing", 5, time.Second)
	}

	err := f.err()
	if err != nil {
		return MSE4ObjIterRecord{}, err
	}

	return record, nil
}

func (r MSE4ObjIterRecord) String() string {
	s := fmt.Sprintf(
		"%s elapsed | %s processed | %s processing | %s backend wait",
		r.TimeElapsed,
		r.BytesProcessed.String(),
		r.TimeProcessing,
		r.TimeBackendWait,
	)

	if r.IsPersisted {
		s += fmt.Sprintf(
			" | %s disk IO bytes | %s disk IO processing",
			r.DiskIOBytes.String(),
			r.TimeDiskIOProcessing,
		)
	}

	return s
}

// MSE4ChunkFaultRecord holds MSE4 persisted chunk memory fault timing data.
type MSE4ChunkFaultRecord struct {
	BaseRecord

	ChunksProcessed      int64         // Number of chunks processed
	BytesProcessed       SizeValue     // Number of bytes processed
	TimeProcessing       time.Duration // Total time spent on processing (seconds)
	TimeMemoryAllocation time.Duration // Time spent allocating memory (seconds)
	TimeDiskIOWait       time.Duration // Time spent waiting for disk IO (seconds)
}

func NewMSE4ChunkFaultRecord(blr BaseRecord) (MSE4ChunkFaultRecord, error) {
	f := newFieldScanner(blr, "MSE4ChunkFaultRecord")
	f.require(5)

	record := MSE4ChunkFaultRecord{
		BaseRecord:           blr,
		ChunksProcessed:      f.int64("chunks processed", 0),
		BytesProcessed:       f.size("bytes processed", 1),
		TimeProcessing:       f.duration("time processing", 2, time.Second),
		TimeMemoryAllocation: f.duration("time memory allocation", 3, time.Second),
		TimeDiskIOWait:       f.duration("time disk io wait", 4, time.Second),
	}

	err := f.err()
	if err != nil {
		return MSE4ChunkFaultRecord{}, err
	}

	return record, nil
}

func (r MSE4ChunkFaultRecord) String() string {
	return fmt.Sprintf(
		"%d chunks | %s processed | %s processing | %s mem alloc | %s disk IO wait",
		r.ChunksProcessed,
		r.BytesProcessed.String(),
		r.TimeProcessing,
		r.TimeMemoryAllocation,
		r.TimeDiskIOWait,
	)
}

// BrotliRecord holds Brotli compression/decompression operation data.
type BrotliRecord struct {
	BaseRecord

	Operation   rune      // 'B': Brotli, 'U': Unbrotli, 'u': Unbrotli-test
	Direction   rune      // 'F': Fetch, 'D': Deliver
	BytesInput  SizeValue // Bytes input
	BytesOutput SizeValue // Bytes output
}

func NewBrotliRecord(blr BaseRecord) (BrotliRecord, error) {
	f := newFieldScanner(blr, "BrotliRecord")
	f.require(4)

	record := BrotliRecord{
		BaseRecord:  blr,
		Operation:   f.rune("operation", 0, 'B', 'U', 'u'),
		Direction:   f.rune("direction", 1, 'F', 'D'),
		BytesInput:  f.size("bytes input", 2),
		BytesOutput: f.size("bytes output", 3),
	}

	err := f.err()
	if err != nil {
		return BrotliRecord{}, err
	}

	return record, nil
}

func (r BrotliRecord) String() string {
	var operation string

	switch r.Operation {
	case 'B':
		operation = "Brotli"
	case 'U':
		operation = "Unbrotli"
	case 'u':
		operation = "Unbrotli-test"
	default:
		operation = string(r.Operation)
	}

	var direction string

	switch r.Direction {
	case 'F':
		direction = "Fetch"
	case 'D':
		direction = "Deliver"
	default:
		direction = string(r.Direction)
	}

	return fmt.Sprintf(
		"%s | %s | %s input | %s output",
		operation,
		direction,
		r.BytesInput.String(),
		r.BytesOutput.String(),
	)
}

/* BaseRecord aliases */

// EndRecord marks the end of a transaction.
type EndRecord struct{ BaseRecord }

// VCLCallRecord is the VCL method called (RECV, DELIVER, BACKEND_FETCH, ...).
type VCLCallRecord struct{ BaseRecord }

// VCLReturnRecord is the VCL method return value (hash, lookup, fetch, deliver, ...).
type VCLReturnRecord struct{ BaseRecord }

// VCLUseRecord is the VCL name in use.
type VCLUseRecord struct{ BaseRecord }

// ReasonRecord is the response reason.
type ReasonRecord struct{ BaseRecord }

// FetchErrorRecord holds the error msg of an error while fetching the object from the backend.
type FetchErrorRecord struct{ BaseRecord }

// MethodRecord holds the method for ReqMethod or BereqMethod tags.
type MethodRecord struct{ BaseRecord }

// ProtocolRecord holds the protocol for the ReqProtocol, RespProtocol, BereqProtocol, ... tags.
type ProtocolRecord struct{ BaseRecord }

// ErrorRecord holds error messages.
type ErrorRecord struct{ BaseRecord }
