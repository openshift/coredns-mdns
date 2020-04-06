package mdns

import (
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
		srvHosts := make(map[string][]*zeroconf.ServiceEntry)
		mutex := sync.RWMutex{}
		m := MDNS{nil, tc.domain, 0, "", &mutex, &hosts, &srvHosts}
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
