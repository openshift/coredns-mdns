package mdns

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"

	"github.com/celebdor/zeroconf"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
)

func makeServiceEntry(name, domain string, ip net.IP) *zeroconf.ServiceEntry {
	var ipAddrv4, ipAddrv6 []net.IP
	if ip.To4() != nil {
		ipAddrv4 = append(ipAddrv4, ip)
	} else {
		ipAddrv6 = append(ipAddrv6, ip)
	}
	return &zeroconf.ServiceEntry{
		ServiceRecord: *zeroconf.NewServiceRecord("My Machine", "_http._tcp.", domain),
		HostName:      name,
		AddrIPv4:      ipAddrv4,
		AddrIPv6:      ipAddrv6,
	}
}

var (
	ipv4 = makeServiceEntry("mymachine", "example.com", net.ParseIP("10.1.1.1"))
	ipv6 = makeServiceEntry("mymachine", "example.com", net.ParseIP("2001::1"))
)

func TestAddARecord(t *testing.T) {
	testCases := []struct {
		tcase          string
		name           string
		domain         string
		ip             net.IP
		responseWriter dns.ResponseWriter
		hosts          map[string]*zeroconf.ServiceEntry
		expected       string
		expectedRet    bool
	}{
		{"valid local ipv4", "mymachine.local", "example.com", net.ParseIP("10.1.1.1"), nilResponseWriter{}, map[string]*zeroconf.ServiceEntry{"mymachine.local": ipv4}, "mymachine.local	60	IN	A	10.1.1.1", true},
		{"valid local ipv6", "mymachine.local", "example.com", net.ParseIP("2001::1"), nilResponseWriter{}, map[string]*zeroconf.ServiceEntry{"mymachine.local": ipv6}, "mymachine.local	60	IN	AAAA	2001::1", true},
		{"local not found", "notthere.local", "example.com", net.ParseIP("10.1.1.1"), nilResponseWriter{}, map[string]*zeroconf.ServiceEntry{"mymachine.local": ipv4}, "", false},
	}
	for _, tc := range testCases {
		hosts := tc.hosts
		mutex := sync.RWMutex{}
		m := MDNS{nil, tc.domain, "", "", &mutex, &hosts}
		msg := new(dns.Msg)
		reply := new(dns.Msg)
		msg.SetReply(reply)
		state := request.Request{W: tc.responseWriter, Req: reply}
		success := m.AddARecord(msg, &state, hosts, tc.name)
		if success != tc.expectedRet {
			t.Errorf("case[%v]: Failed", tc.tcase)
		}
		if success && msg.Answer[0].String() != tc.expected {
			t.Errorf("case[%v]: expected %v, got %+v", tc.tcase, tc.expected, msg.Answer[0].String())
		}
	}
}

type nilResponseWriter struct {
}

func (nilResponseWriter) LocalAddr() net.Addr {
	return nil
}

func (nilResponseWriter) RemoteAddr() net.Addr {
	return nil
}

func (nilResponseWriter) Close() error {
	return nil
}

func (nilResponseWriter) Hijack() {
}

func (nilResponseWriter) TsigStatus() error {
	return nil
}

func (nilResponseWriter) WriteMsg(msg *dns.Msg) error {
	return nil
}

func (nilResponseWriter) Write(b []byte) (int, error) {
	return 0, nil
}

func (nilResponseWriter) TsigTimersOnly(b bool) {
}

func TestQueryService(t *testing.T) {
	testCases := []struct {
		tcase         string
		expectedError string
		zeroconfImpl  ZeroconfInterface
	}{
		{"queryService succeeds", "", fakeZeroconf{}},
		{"NewResolver fails", "test resolver error", failZeroconf{}},
		{"Browse fails", "test browse error", browseFailZeroconf{}},
	}
	for _, tc := range testCases {
		entriesCh := make(chan *zeroconf.ServiceEntry)
		result := queryService("test", entriesCh, net.Interface{}, tc.zeroconfImpl)
		if tc.expectedError == "" {
			if result != nil {
				t.Errorf("Unexpected failure in %v: %v", tc.tcase, result)
			}
		} else {
			if result.Error() != tc.expectedError {
				t.Errorf("Unexpected result in %v: %v", tc.tcase, result)
			}
		}
	}
}

type fakeZeroconf struct{}

func (fakeZeroconf) NewResolver(opts ...zeroconf.ClientOption) (ResolverInterface, error) {
	return fakeResolver{}, nil
}

type failZeroconf struct{}

func (failZeroconf) NewResolver(opts ...zeroconf.ClientOption) (ResolverInterface, error) {
	return nil, errors.New("test resolver error")
}

type fakeResolver struct{}

func (fakeResolver) Browse(context context.Context, service, domain string, entries chan<- *zeroconf.ServiceEntry) error {
	return nil
}

type browseFailZeroconf struct{}

func (browseFailZeroconf) NewResolver(opts ...zeroconf.ClientOption) (ResolverInterface, error) {
	return failResolver{}, nil
}

type failResolver struct{}

func (failResolver) Browse(context context.Context, service, domain string, entries chan<- *zeroconf.ServiceEntry) error {
	return errors.New("test browse error")
}

func TestReplaceDomain(t *testing.T) {
	testCases := []struct {
		tcase    string
		input    string
		expected string
	}{
		{".local", "foo.local.", "foo.testdomain."},
		{".bar", "foo.bar.", "foo.testdomain."},
		{".foobar", "foo.foobar.", "foo.testdomain."},
		{".bar.baz", "foo.bar.baz.", "foo.testdomain."},
	}

	mdnsHosts := make(map[string]*zeroconf.ServiceEntry)
	mutex := sync.RWMutex{}
	m := MDNS{Domain: "testdomain", filter: "", bindAddress: "", mutex: &mutex, mdnsHosts: &mdnsHosts}
	for _, tc := range testCases {
		result := m.ReplaceDomain(tc.input)
		if result != tc.expected {
			t.Errorf("Incorrect domain replacement in %s: '%s' != '%s'", tc.tcase, tc.expected, result)
		}
	}
}
