package mdns

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/coredns/coredns/plugin"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/request"

	"github.com/grandcat/zeroconf"
	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

var log = clog.NewWithPlugin("mdns")

type MDNS struct {
	Next      plugin.Handler
	filter    string
	mutex     *sync.RWMutex
	mdnsHosts *map[string]*zeroconf.ServiceEntry
	srvHosts  *map[string][]*zeroconf.ServiceEntry
}

func (m MDNS) AddARecord(msg *dns.Msg, state *request.Request, hosts map[string]*zeroconf.ServiceEntry, name string) bool {
	// Add A and AAAA record for name (if it exists) to msg.
	// A records need to be returned in A queries, this function
	// provides common code for doing so.
	answerEntry, present := hosts[name]
	if present {
		if answerEntry.AddrIPv4 != nil {
			aheader := dns.RR_Header{Name: name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60}
			// TODO: Support multiple addresses
			msg.Answer = append(msg.Answer, &dns.A{Hdr: aheader, A: answerEntry.AddrIPv4[0]})
		}
		if answerEntry.AddrIPv6 != nil {
			aaaaheader := dns.RR_Header{Name: name, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 60}
			msg.Answer = append(msg.Answer, &dns.AAAA{Hdr: aaaaheader, AAAA: answerEntry.AddrIPv6[0]})
		}
		return true
	}
	return false
}

func (m MDNS) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {

	log.Debug("Received query")
	msg := new(dns.Msg)
	msg.SetReply(r)
	msg.Authoritative = true
	msg.RecursionAvailable = true
	state := request.Request{W: w, Req: r}
	log.Debugf("Looking for name: %s", state.QName())
	// Just for convenience so we don't have to keep dereferencing these
	mdnsHosts := *m.mdnsHosts
	srvHosts := *m.srvHosts

	if !strings.HasSuffix(state.QName(), "local.") {
		log.Debugf("Skipping due to query '%s' not '.local'", state.QName())
		return plugin.NextOrFailure(m.Name(), m.Next, ctx, w, r)
	}

	if state.QType() != dns.TypeA && state.QType() != dns.TypeAAAA && state.QType() != dns.TypeSRV {
		log.Debugf("Skipping due to unrecognized query type %v", state.QType())
		return plugin.NextOrFailure(m.Name(), m.Next, ctx, w, r)
	}

	msg.Answer = []dns.RR{}
	hostName := strings.ToLower(state.QName)

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.AddARecord(msg, &state, mdnsHosts, hostName) {
		log.Debug(msg)
		w.WriteMsg(msg)
		return dns.RcodeSuccess, nil
	}

	srvEntry, present := srvHosts[hostName]
	if present {
		srvheader := dns.RR_Header{Name: state.QName(), Rrtype: dns.TypeSRV, Class: dns.ClassINET, Ttl: 0}
		for _, host := range srvEntry {
			msg.Answer = append(msg.Answer, &dns.SRV{Hdr: srvheader, Target: host.HostName, Priority: 0, Weight: 10, Port: uint16(host.Port)})
		}
		log.Debug(msg)
		w.WriteMsg(msg)
		return dns.RcodeSuccess, nil
	}
	log.Debugf("No records found for '%s', forwarding to next plugin.", state.QName())
	return plugin.NextOrFailure(m.Name(), m.Next, ctx, w, r)
}

func (m *MDNS) BrowseMDNS() {
	entriesCh := make(chan *zeroconf.ServiceEntry)
	srvEntriesCh := make(chan *zeroconf.ServiceEntry)
	mdnsHosts := make(map[string]*zeroconf.ServiceEntry)
	srvHosts := make(map[string][]*zeroconf.ServiceEntry)
	go func(results <-chan *zeroconf.ServiceEntry) {
		log.Debug("Retrieving mDNS entries")
		for entry := range results {
			// Make a copy of the entry so zeroconf can't later overwrite our changes
			localEntry := *entry
			log.Debugf("A Instance: %s, HostName: %s, AddrIPv4: %s, AddrIPv6: %s\n", localEntry.Instance, localEntry.HostName, localEntry.AddrIPv4, localEntry.AddrIPv6)
			if strings.Contains(localEntry.Instance, m.filter) {
				hostName := strings.ToLower(localEntry.HostName)
				mdnsHosts[hostName] = entry
			} else {
				log.Debugf("Ignoring entry '%s' because it doesn't match filter '%s'\n",
					localEntry.Instance, m.filter)
			}
		}
	}(entriesCh)

	go func(results <-chan *zeroconf.ServiceEntry) {
		log.Debug("Retrieving SRV mDNS entries")
		for entry := range results {
			// Make a copy of the entry so mdns can't later overwrite our changes
			localEntry := *entry
			log.Debugf("SRV Instance: %s, Service: %s, Domain: %s, HostName: %s, AddrIPv4: %s, AddrIPv6: %s\n", localEntry.Instance, localEntry.Service, localEntry.Domain, localEntry.HostName, localEntry.AddrIPv4, localEntry.AddrIPv6)
			if strings.Contains(localEntry.Instance, m.filter) {
				hostName := strings.ToLower(localEntry.HostName)
				srvHosts[hostName] = append(srvHosts[hostName], &localEntry)
			} else {
				log.Debugf("Ignoring entry '%s' because it doesn't match filter '%s'\n",
					localEntry.Instance, m.filter)
			}
		}
	}(srvEntriesCh)

	queryService("_workstation._tcp", entriesCh)
	//queryService("_etcd-server-ssl._tcp", srvEntriesCh)

	m.mutex.Lock()
	defer m.mutex.Unlock()
	// Clear maps so we don't have stale entries
	for k := range *m.mdnsHosts {
		delete(*m.mdnsHosts, k)
	}
	for k := range *m.srvHosts {
		delete(*m.srvHosts, k)
	}
	// Copy values into the shared maps only after we've collected all of them.
	// This prevents us from having to lock during the entire mdns browse time.
	for k, v := range mdnsHosts {
		(*m.mdnsHosts)[k] = v
	}
	for k, v := range srvHosts {
		// Don't return any SRV records until we have enough of them. Returning
		// partial SRV lists can result in bad clustering.
		(*m.srvHosts)[k] = v
	}
	log.Debugf("mdnsHosts: %v", m.mdnsHosts)
	for name, entry := range *m.mdnsHosts {
		log.Debugf("%s: %v", name, entry)
	}
	log.Debugf("srvHosts: %v", m.srvHosts)
	for name, records := range *m.srvHosts {
		for _, v := range records {
			log.Debugf("%s: %v", name, v)
		}
	}
}

func queryService(service string, channel chan *zeroconf.ServiceEntry) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		log.Errorf("Failed to initialize %s resolver: %s", service, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err = resolver.Browse(ctx, service, "local.", channel)
	if err != nil {
		log.Errorf("Failed to browse %s records: %s", service, err.Error())
		return
	}
	<-ctx.Done()
}

func (m MDNS) Name() string { return "mdns" }

type ResponsePrinter struct {
	dns.ResponseWriter
}

func NewResponsePrinter(w dns.ResponseWriter) *ResponsePrinter {
	return &ResponsePrinter{ResponseWriter: w}
}

func (r *ResponsePrinter) WriteMsg(res *dns.Msg) error {
	fmt.Fprintln(out, m)
	return r.ResponseWriter.WriteMsg(res)
}

var out io.Writer = os.Stdout

const m = "mdns"
