package vsl_test

import (
	"net"
	"testing"
	"time"

	"github.com/aorith/varnishlog-parser/vsl"
)

func TestBaseRecord(t *testing.T) {
	type test struct {
		logRecord string
		tag       string
		value     string
	}

	testList := []test{
		{
			logRecord: "--  ReqHeader      Host: www.example1.com",
			tag:       "ReqHeader",
			value:     "Host: www.example1.com",
		},
		{
			logRecord: "--  BereqHeader      User-Agent:curl/8.9.1 ---- as   012345.",
			tag:       "BereqHeader",
			value:     "User-Agent:curl/8.9.1 ---- as   012345.",
		},
		{
			logRecord: "-   End",
			tag:       "End",
			value:     "",
		},
	}

	for _, test := range testList {
		record, err := vsl.NewBaseRecord(test.logRecord)
		if err != nil {
			t.Errorf("conversion to BaseRecord failed: %s", err)
		}
		if record.Tag() != test.tag {
			t.Errorf("Expected tag %q got %q", test.tag, record.Tag())
		}
		if record.Value() != test.value {
			t.Errorf("Expected value %q got %q", test.value, record.Value())
		}
	}
}

func TestHeaders(t *testing.T) {
	type test struct {
		logRecord   string
		header      string
		headerValue string
	}

	testList := []test{
		{
			logRecord:   "--  ReqHeader      Host: www.example1.com",
			header:      "Host",
			headerValue: "www.example1.com",
		},
		{
			logRecord:   "--  ReqHeader      User-Agent:curl/8.9.1 ---- as   012345.",
			header:      "User-Agent",
			headerValue: "curl/8.9.1 ---- as   012345.",
		},
	}

	for _, test := range testList {
		blr, err := vsl.NewBaseRecord(test.logRecord)
		if err != nil {
			t.Errorf("conversion to BaseRecord failed: %s", err)
		}
		record, err := vsl.NewHeaderRecord(blr)
		if err != nil {
			t.Errorf("conversion to HeaderRecord failed: %s", err)
		}
		if record.Name() != test.header {
			t.Errorf("Expected header %q got %q", test.header, record.Name())
		}
		if record.Value() != test.headerValue {
			t.Errorf("Expected header value %q got %q", test.headerValue, record.Value())
		}
	}
}

func TestBeginRecord(t *testing.T) {
	type test struct {
		logRecord        string
		recordType       string
		parentVXID       vsl.VXID
		reasonOrProtocol string
		esiLevel         int
	}

	testList := []test{
		{
			logRecord:        "-4- Begin          req 32772 esi 2",
			recordType:       "req",
			parentVXID:       32772,
			reasonOrProtocol: "esi",
			esiLevel:         2,
		},
		{
			logRecord:        "-5- Begin          bereq 32774 fetch",
			recordType:       "bereq",
			parentVXID:       32774,
			reasonOrProtocol: "fetch",
			esiLevel:         0,
		},
		{
			logRecord:        "-   Begin          sess 0 HTTP/1",
			recordType:       "sess",
			parentVXID:       0,
			reasonOrProtocol: "HTTP/1",
			esiLevel:         0,
		},
	}

	for _, test := range testList {
		blr, err := vsl.NewBaseRecord(test.logRecord)
		if err != nil {
			t.Errorf("conversion to BaseRecord failed: %s", err)
		}
		record, err := vsl.NewBeginRecord(blr)
		if err != nil {
			t.Errorf("conversion failed: %s", err)
		}
		if record.Type() != test.recordType {
			t.Errorf("Expected type %q got %q", test.recordType, record.Type())
		}
		if record.ParentVXID() != test.parentVXID {
			t.Errorf("Expected parentVXID %d got %d", test.parentVXID, record.ParentVXID())
		}
		if record.ReasonOrProtocol() != test.reasonOrProtocol {
			t.Errorf("Expected ReasonOrProtocol %q got %q", test.reasonOrProtocol, record.ReasonOrProtocol())
		}
		if record.ESILevel() != test.esiLevel {
			t.Errorf("Expected ESILevel %d got %d", test.esiLevel, record.ESILevel())
		}
	}
}

func TestLinkRecord(t *testing.T) {
	type test struct {
		logRecord string
		txid      string
		txType    string
		vxid      vsl.VXID
		reason    string
		esiLevel  int
	}

	testList := []test{
		{
			logRecord: "-   Link           req 32770 rxreq",
			txid:      "32770_req",
			txType:    "req",
			vxid:      32770,
			reason:    "rxreq",
			esiLevel:  0,
		},
		{
			logRecord: "--  Link           bereq 32771 fetch",
			txid:      "32771_bereq",
			txType:    "bereq",
			vxid:      32771,
			reason:    "fetch",
			esiLevel:  0,
		},
		{
			logRecord: "--  Link           req 32772 esi 1",
			txid:      "32772_req_esi_1",
			txType:    "req",
			vxid:      32772,
			reason:    "esi",
			esiLevel:  1,
		},
	}

	for _, test := range testList {
		blr, err := vsl.NewBaseRecord(test.logRecord)
		if err != nil {
			t.Errorf("conversion to BaseRecord failed: %s", err)
		}
		record, err := vsl.NewLinkRecord(blr)
		if err != nil {
			t.Errorf("conversion failed: %s", err)
		}
		if record.TXID() != test.txid {
			t.Errorf("TXID() want: %q got: %q", test.txid, record.TXID())
		}
		if record.Type() != test.txType {
			t.Errorf("Expected type %q got %q", test.txType, record.Type())
		}
		if record.VXID() != test.vxid {
			t.Errorf("Expected childVXID %d got %d", test.vxid, record.VXID())
		}
		if record.Reason() != test.reason {
			t.Errorf("Expected reason %q got %q", test.reason, record.Reason())
		}
		if record.ESILevel() != test.esiLevel {
			t.Errorf("Expected ESILevel %d got %d", test.esiLevel, record.ESILevel())
		}
	}
}

func TestBackendOpenRecord(t *testing.T) {
	type test struct {
		logRecord      string
		fileDescriptor int
		name           string
		remoteAddr     net.IP
		remotePort     int
		localAddr      net.IP
		localPort      int
		reason         string
	}

	testList := []test{
		{
			logRecord:      "--- BackendOpen    29 varnishb 192.168.50.11 80 192.168.50.10 51776 connect",
			fileDescriptor: 29,
			name:           "varnishb",
			remoteAddr:     net.IPv4(192, 168, 50, 11),
			remotePort:     80,
			localAddr:      net.IPv4(192, 168, 50, 10),
			localPort:      51776,
			reason:         "connect",
		},
		{
			logRecord:      "--- BackendOpen    29 varnishA 192.168.50.9 80 192.168.50.10 51778",
			fileDescriptor: 29,
			name:           "varnishA",
			remoteAddr:     net.IPv4(192, 168, 50, 9),
			remotePort:     80,
			localAddr:      net.IPv4(192, 168, 50, 10),
			localPort:      51778,
			reason:         "-",
		},
	}

	for _, test := range testList {
		blr, err := vsl.NewBaseRecord(test.logRecord)
		if err != nil {
			t.Errorf("conversion to BaseRecord failed: %s", err)
		}
		record, err := vsl.NewBackendOpenRecord(blr)
		if err != nil {
			t.Errorf("conversion failed: %s", err)
		}
		if record.FileDescriptor() != test.fileDescriptor {
			t.Errorf("Expected fileDescriptor %v got %v", test.fileDescriptor, record.FileDescriptor())
		}
		if record.Name() != test.name {
			t.Errorf("Expected name %v got %v", test.name, record.Name())
		}
		if !test.remoteAddr.Equal(record.RemoteAddr()) {
			t.Errorf("Expected remoteAddr %v got %v", test.remoteAddr, record.RemoteAddr())
		}
		if record.RemotePort() != test.remotePort {
			t.Errorf("Expected remotePort %v got %v", test.remotePort, record.RemotePort())
		}
		if !test.localAddr.Equal(record.LocalAddr()) {
			t.Errorf("Expected localAddr %v got %v", test.localAddr, record.LocalAddr())
		}
		if record.LocalPort() != test.localPort {
			t.Errorf("Expected localPort %v got %v", test.localPort, record.LocalPort())
		}
		if record.Reason() != test.reason {
			t.Errorf("Expected reason %v got %v", test.reason, record.Reason())
		}
	}
}

func TestBackendCloseRecord(t *testing.T) {
	type test struct {
		logRecord      string
		fileDescriptor int
		name           string
		reason         string
	}

	testList := []test{
		{
			logRecord:      "-5- BackendClose   29 varnishb recycle",
			fileDescriptor: 29,
			name:           "varnishb",
			reason:         "recycle",
		},
		{
			logRecord:      "-5- BackendClose   30 varnishA",
			fileDescriptor: 30,
			name:           "varnishA",
			reason:         "unknown",
		},
	}

	for _, test := range testList {
		blr, err := vsl.NewBaseRecord(test.logRecord)
		if err != nil {
			t.Errorf("conversion to BaseRecord failed: %s", err)
		}
		record, err := vsl.NewBackendCloseRecord(blr)
		if err != nil {
			t.Errorf("conversion failed: %s", err)
		}
		if record.FileDescriptor() != test.fileDescriptor {
			t.Errorf("Expected fileDescriptor %v got %v", test.fileDescriptor, record.FileDescriptor())
		}
		if record.Name() != test.name {
			t.Errorf("Expected name %v got %v", test.name, record.Name())
		}
		if record.Reason() != test.reason {
			t.Errorf("Expected reason %v got %v", test.reason, record.Reason())
		}
	}
}

func TestAcctRecord(t *testing.T) {
	type test struct {
		logRecord string
		headerTx  vsl.SizeValue
		bodyTx    vsl.SizeValue
		totalTx   vsl.SizeValue
		headerRx  vsl.SizeValue
		bodyRx    vsl.SizeValue
		totalRx   vsl.SizeValue
	}

	testList := []test{
		{
			logRecord: "-4- BereqAcct      234 0 234 171 40 211",
			headerTx:  vsl.SizeValue(234),
			bodyTx:    vsl.SizeValue(0),
			totalTx:   vsl.SizeValue(234),
			headerRx:  vsl.SizeValue(171),
			bodyRx:    vsl.SizeValue(40),
			totalRx:   vsl.SizeValue(211),
		},
		{
			logRecord: "--  ReqAcct        84 0 84 279 100 379",
			headerTx:  vsl.SizeValue(84),
			bodyTx:    vsl.SizeValue(0),
			totalTx:   vsl.SizeValue(84),
			headerRx:  vsl.SizeValue(279),
			bodyRx:    vsl.SizeValue(100),
			totalRx:   vsl.SizeValue(379),
		},
	}

	for _, test := range testList {
		blr, err := vsl.NewBaseRecord(test.logRecord)
		if err != nil {
			t.Errorf("conversion to BaseRecord failed: %s", err)
		}
		record, err := vsl.NewAcctRecord(blr)
		if err != nil {
			t.Errorf("conversion failed: %s", err)
		}
		if record.HeaderTx() != test.headerTx {
			t.Errorf("Expected headerTx %v got %v", test.headerTx, record.HeaderTx())
		}
		if record.BodyTx() != test.bodyTx {
			t.Errorf("Expected bodyTx %v got %v", test.bodyTx, record.BodyTx())
		}
		if record.TotalTx() != test.totalTx {
			t.Errorf("Expected totalTx %v got %v", test.totalTx, record.TotalTx())
		}
		if record.HeaderRx() != test.headerRx {
			t.Errorf("Expected headerRx %v got %v", test.headerRx, record.HeaderRx())
		}
		if record.BodyRx() != test.bodyRx {
			t.Errorf("Expected bodyRx %v got %v", test.bodyRx, record.BodyRx())
		}
		if record.TotalRx() != test.totalRx {
			t.Errorf("Expected totalRx %v got %v", test.totalRx, record.TotalRx())
		}
	}
}

func TestTimestampRecord(t *testing.T) {
	type test struct {
		logRecord    string
		eventLabel   string
		absoluteTime time.Time
		sinceStart   time.Duration
		sinceLast    time.Duration
	}

	testList := []test{
		{
			logRecord:    "--  Timestamp      Process: 1728889150.269166 0.000768 0.000009",
			eventLabel:   "Process",
			absoluteTime: time.Unix(1728889150, 269166*1000),
			sinceStart:   time.Duration(float64(0.000768) * float64(time.Second)),
			sinceLast:    time.Duration(float64(0.000009) * float64(time.Second)),
		},
		{
			// The absolute time in this example is actually unrealistic since Varnish should always
			// add the 6 digits for the microseconds
			logRecord:    "--- Timestamp      BerespBody: 1728889150.2501 0.001656 0.000183",
			eventLabel:   "BerespBody",
			absoluteTime: time.Unix(1728889150, 250100*1000),
			sinceStart:   time.Duration(float64(0.001656) * float64(time.Second)),
			sinceLast:    time.Duration(float64(0.000183) * float64(time.Second)),
		},
		{
			logRecord:    "-   Timestamp      Process: 1729152215.071000 0.002625 0.002605",
			eventLabel:   "Process",
			absoluteTime: time.Unix(1729152215, 71000*1000),
			sinceStart:   time.Duration(float64(0.002625) * float64(time.Second)),
			sinceLast:    time.Duration(float64(0.002605) * float64(time.Second)),
		},
	}

	for _, test := range testList {
		blr, err := vsl.NewBaseRecord(test.logRecord)
		if err != nil {
			t.Errorf("conversion to BaseRecord failed: %s", err)
		}
		record, err := vsl.NewTimestampRecord(blr)
		if err != nil {
			t.Errorf("conversion failed: %s", err)
		}
		if record.EventLabel() != test.eventLabel {
			t.Errorf("Expected eventLabel %v got %v", test.eventLabel, record.EventLabel())
		}
		if record.AbsoluteTime() != test.absoluteTime {
			t.Errorf("Expected absoluteTime %v got %v", test.absoluteTime, record.AbsoluteTime())
		}
		if record.SinceStart() != test.sinceStart {
			t.Errorf("Expected sinceStart %v got %v", test.sinceStart, record.SinceStart())
		}
		if record.SinceLast() != test.sinceLast {
			t.Errorf("Expected sinceLast %v got %v", test.sinceLast, record.SinceLast())
		}
	}
}

func TestURLRecord(t *testing.T) {
	type test struct {
		logRecord string
		path      string
		query     string
	}

	testList := []test{
		{
			logRecord: "-4- ReqURL         /last-content/media?a=1&b=2",
			path:      "/last-content/media",
			query:     "a=1&b=2",
		},
		{
			logRecord: "-5- BereqURL       /this/is/a/long/path",
			path:      "/this/is/a/long/path",
			query:     "",
		},
		{
			logRecord: "-5- BereqURL       /this/is/a/even/more/long/path/right/here/?abc=1&cba=2",
			path:      "/this/is/a/even/more/long/path/right/here/",
			query:     "abc=1&cba=2",
		},
	}

	for _, test := range testList {
		blr, err := vsl.NewBaseRecord(test.logRecord)
		if err != nil {
			t.Errorf("conversion to BaseRecord failed: %s", err)
		}
		record, err := vsl.NewURLRecord(blr)
		if err != nil {
			t.Errorf("conversion failed: %s", err)
		}

		if record.Path() != test.path {
			t.Errorf("Path() want: %q got: %q", test.path, record.Path())
		}
		if record.Query() != test.query {
			t.Errorf("Query() want: %q got: %q", test.query, record.Query())
		}
	}
}

func TestHitRecord(t *testing.T) {
	type test struct {
		logRecord string
		vxid      vsl.VXID
		ttl       time.Duration
		grace     time.Duration
		keep      time.Duration
	}

	testList := []test{
		{
			logRecord: "-4- Hit            32775 14.998964 10.000000 0.000000",
			vxid:      vsl.VXID(32775),
			ttl:       time.Duration(float64(14.998964) * float64(time.Second)),
			grace:     time.Duration(10 * time.Second),
			keep:      time.Duration(0 * time.Second),
		},
		{
			logRecord: "-4- Hit            32775 14.998964 10.000000 0.000000 a b",
			vxid:      vsl.VXID(32775),
			ttl:       time.Duration(float64(14.998964) * float64(time.Second)),
			grace:     time.Duration(10 * time.Second),
			keep:      time.Duration(0 * time.Second),
		},
	}

	for _, test := range testList {
		blr, err := vsl.NewBaseRecord(test.logRecord)
		if err != nil {
			t.Errorf("conversion to BaseRecord failed: %s", err)
		}
		record, err := vsl.NewHitRecord(blr)
		if err != nil {
			t.Errorf("conversion failed: %s", err)
		}

		if record.ObjVXID() != test.vxid {
			t.Errorf("ObjVXID() want: %v got: %v", test.vxid, record.ObjVXID())
		}
		if record.TTL() != test.ttl {
			t.Errorf("TTL() want: %v got: %v", test.ttl, record.TTL())
		}
		if record.Grace() != test.grace {
			t.Errorf("Grace() want: %v got: %v", test.grace, record.Grace())
		}
		if record.Keep() != test.keep {
			t.Errorf("Keep() want: %v got: %v", test.keep, record.Keep())
		}
	}
}

func TestTTLRecord(t *testing.T) {
	testList := []struct {
		logRecord   string
		source      string
		ttl         time.Duration
		grace       time.Duration
		keep        time.Duration
		reference   time.Time
		age         time.Time
		date        time.Time
		expires     time.Time
		maxAge      time.Duration
		cacheStatus string
	}{
		{
			logRecord:   "-5- TTL            RFC 120 10 0 1728889150 1728889150 1728889150 0 0 cacheable",
			source:      "RFC",
			ttl:         time.Duration(120 * time.Second),
			grace:       time.Duration(10 * time.Second),
			keep:        time.Duration(0 * time.Second),
			reference:   time.Unix(1728889150, 0),
			age:         time.Unix(1728889150, 0),
			date:        time.Unix(1728889150, 0),
			expires:     time.Unix(0, 0),
			maxAge:      time.Duration(0),
			cacheStatus: "cacheable",
		},
		{
			logRecord:   "-   TTL            RFC 60 10 0 1729179266 1729179266 1729179266 1729179301 60 cacheable",
			source:      "RFC",
			ttl:         time.Duration(60 * time.Second),
			grace:       time.Duration(10 * time.Second),
			keep:        time.Duration(0 * time.Second),
			reference:   time.Unix(1729179266, 0),
			age:         time.Unix(1729179266, 0),
			date:        time.Unix(1729179266, 0),
			expires:     time.Unix(1729179301, 0),
			maxAge:      time.Duration(60 * time.Second),
			cacheStatus: "cacheable",
		},
		{
			logRecord:   "-   TTL            VCL 90 1 25200 1729179266 cacheable",
			source:      "VCL",
			ttl:         time.Duration(90 * time.Second),
			grace:       time.Duration(1 * time.Second),
			keep:        time.Duration(25200 * time.Second),
			reference:   time.Unix(1729179266, 0),
			cacheStatus: "cacheable",
		},
	}

	for _, test := range testList {
		blr, err := vsl.NewBaseRecord(test.logRecord)
		if err != nil {
			t.Errorf("conversion to BaseRecord failed: %s", err)
		}
		record, err := vsl.NewTTLRecord(blr)
		if err != nil {
			t.Errorf("conversion failed: %s", err)
		}

		if record.RawLog() != test.logRecord {
			t.Errorf("RawLog() want: %q got: %q", test.logRecord, record.RawLog())
		}
		if record.Source() != test.source {
			t.Errorf("Source() want: %v got: %v", test.source, record.Source())
		}
		if record.TTL() != test.ttl {
			t.Errorf("TTL() want: %v got: %v", test.ttl, record.TTL())
		}
		if record.Grace() != test.grace {
			t.Errorf("Grace() want: %v got: %v", test.grace, record.Grace())
		}
		if record.Keep() != test.keep {
			t.Errorf("Keep() want: %v got: %v", test.keep, record.Keep())
		}
		if record.Reference() != test.reference {
			t.Errorf("Reference() want: %v got: %v", test.reference, record.Reference())
		}
		if record.Age() != test.age {
			t.Errorf("Age() want: %v got: %v", test.age, record.Age())
		}
		if record.Date() != test.date {
			t.Errorf("Date() want: %v got: %v", test.date, record.Date())
		}
		if record.Expires() != test.expires {
			t.Errorf("Expires() want: %v got: %v", test.expires, record.Expires())
		}
		if record.MaxAge() != test.maxAge {
			t.Errorf("MaxAge() want: %v got: %v", test.maxAge, record.MaxAge())
		}
		if record.CacheStatus() != test.cacheStatus {
			t.Errorf("CacheStatus() want: %v got: %v", test.cacheStatus, record.CacheStatus())
		}
	}
}

func TestSessOpenRecord(t *testing.T) {
	testList := []struct {
		logRecord      string
		remoteAddr     net.IP
		remotePort     int
		socketName     string
		localAddr      net.IP
		localPort      int
		sessionStart   time.Time
		fileDescriptor int
	}{
		{
			logRecord:      "-   SessOpen       192.168.50.1 55666 http 192.168.50.10 80 1728889150.268365 23",
			remoteAddr:     net.ParseIP("192.168.50.1"),
			remotePort:     55666,
			socketName:     "http",
			localAddr:      net.ParseIP("192.168.50.10"),
			localPort:      80,
			sessionStart:   time.Unix(1728889150, 268365*1e3),
			fileDescriptor: 23,
		},
		{
			logRecord:      "-   SessOpen       192.168.50.1 55678 http 192.168.50.10 80 1728889150.793100 28",
			remoteAddr:     net.ParseIP("192.168.50.1"),
			remotePort:     55678,
			socketName:     "http",
			localAddr:      net.ParseIP("192.168.50.10"),
			localPort:      80,
			sessionStart:   time.Unix(1728889150, 793100*1e3),
			fileDescriptor: 28,
		},
	}

	for _, test := range testList {
		blr, err := vsl.NewBaseRecord(test.logRecord)
		if err != nil {
			t.Errorf("conversion to BaseRecord failed: %s", err)
		}

		record, err := vsl.NewSessOpenRecord(blr)
		if err != nil {
			t.Errorf("conversion to SessOpenRecord failed: %s", err)
		}

		if !record.RemoteAddr().Equal(test.remoteAddr) {
			t.Errorf("RemoteAddr() want: %v got: %v", test.remoteAddr, record.RemoteAddr())
		}
		if record.RemotePort() != test.remotePort {
			t.Errorf("RemotePort() want: %v got: %v", test.remotePort, record.RemotePort())
		}
		if record.SocketName() != test.socketName {
			t.Errorf("SocketName() want: %v got: %v", test.socketName, record.SocketName())
		}
		if !record.LocalAddr().Equal(test.localAddr) {
			t.Errorf("LocalAddr() want: %v got: %v", test.localAddr, record.LocalAddr())
		}
		if record.LocalPort() != test.localPort {
			t.Errorf("LocalPort() want: %v got: %v", test.localPort, record.LocalPort())
		}
		if !record.SessionStart().Equal(test.sessionStart) {
			t.Errorf("SessionStart() want: %v got: %v", test.sessionStart, record.SessionStart())
		}
		if record.FileDescriptor() != test.fileDescriptor {
			t.Errorf("FileDescriptor() want: %v got: %v", test.fileDescriptor, record.FileDescriptor())
		}
	}
}

func TestTimeoutRecord(t *testing.T) {
	testList := []struct {
		logRecord string
		reason    string
		duration  time.Duration
	}{
		{
			logRecord: "-   SessClose      REM_CLOSE 0.001",
			reason:    "REM_CLOSE",
			duration:  time.Duration(float64(0.001) * float64(time.Second)),
		},
		{
			logRecord: "-   SessClose      VCL_FAILURE 1.004",
			reason:    "VCL_FAILURE",
			duration:  time.Duration(float64(1.004) * float64(time.Second)),
		},
	}

	for _, test := range testList {
		blr, err := vsl.NewBaseRecord(test.logRecord)
		if err != nil {
			t.Errorf("conversion to BaseRecord failed: %s", err)
		}

		record, err := vsl.NewSessCloseRecord(blr)
		if err != nil {
			t.Errorf("conversion to SessCloseRecord failed: %s", err)
		}

		if record.Reason() != test.reason {
			t.Errorf("Reason() want: %v got: %v", test.reason, record.Reason())
		}
		if record.Duration() != test.duration {
			t.Errorf("Duration() want: %v got: %v", test.duration, record.Duration())
		}
	}
}
