package mdns

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/coredns/coredns/plugin"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/request"

	"github.com/hashicorp/mdns"
	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

var log = clog.NewWithPlugin("mdns")

// Type to sort entries by their Host attribute
type byHost []*mdns.ServiceEntry

func (s byHost) Len() int {
	return len(s)
}

func (s byHost) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s byHost) Less(i, j int) bool {
	return s[i].Host < s[j].Host
}

type MDNS struct {
	Next   plugin.Handler
	Domain string
	mutex *sync.RWMutex
	mdnsHosts *map[string]*mdns.ServiceEntry
	srvHosts *map[string][]*mdns.ServiceEntry
}

func (m MDNS) ReplaceLocal(input string) string {
	// Replace .local domain with our configured custom domain
	fqDomain := "." + strings.TrimSuffix(m.Domain, ".") + "."
	return input[0:len(input)-7] + fqDomain
}

func (m MDNS) AddARecord(msg *dns.Msg, state *request.Request, hosts map[string]*mdns.ServiceEntry, name string) bool {
	// Add A and AAAA record for name (if it exists) to msg.
	// A records need to be returned in both A and CNAME queries, this function
	// provides common code for doing so.
	answerEntry, present := hosts[name]
	if present {
		aheader := dns.RR_Header{Name: state.QName(), Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60}
		msg.Answer = append(msg.Answer, &dns.A{Hdr: aheader, A: answerEntry.AddrV4})
		aaaaheader := dns.RR_Header{Name: state.QName(), Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 60}
		msg.Answer = append(msg.Answer, &dns.AAAA{Hdr: aaaaheader, AAAA: answerEntry.AddrV6})
		return true
	}
	return false
}

func (m MDNS) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {

	log.Debug("Received query")
	msg := new(dns.Msg)
	msg.SetReply(r)
	state := request.Request{W: w, Req: r}
	unqualifiedDomain := strings.TrimSuffix(m.Domain, ".")
	// Just for convenience so we don't have to keep dereferencing these
	mdnsHosts := *m.mdnsHosts
	srvHosts := *m.srvHosts

	if !strings.HasSuffix(state.QName(), unqualifiedDomain + ".") {
		log.Debug("Skipping due to query not in our domain")
		return plugin.NextOrFailure(m.Name(), m.Next, ctx, w, r)
	}

	if state.QType() != dns.TypeA && state.QType() != dns.TypeAAAA && state.QType() != dns.TypeSRV && state.QType() != dns.TypeCNAME {
		log.Debug("Skipping due to unrecognized query type")
		return plugin.NextOrFailure(m.Name(), m.Next, ctx, w, r)
	}

	msg.Answer = []dns.RR{}

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.AddARecord(msg, &state, mdnsHosts, state.Name()) {
		log.Debug(msg)
		w.WriteMsg(msg)
		return dns.RcodeSuccess, nil
	}

	// Create CNAME mapping etcd-X.domain -> master-X.domain
	cnames := make(map[string]string)
	for _, entry := range srvHosts {
		// We need this sorted so our CNAME indices are stable
		sort.Sort(byHost(entry))
		for i, host := range entry {
			_, present := mdnsHosts[host.Host]
			// Ignore entries that point to hosts we don't know about
			if present {
				cname := "etcd-" + strconv.Itoa(i) + "." + unqualifiedDomain + "."
				cnames[cname] = host.Host
			}
		}
	}
	log.Debug(cnames)
	cnameTarget, present := cnames[state.Name()]
	if present {
		cnameheader := dns.RR_Header{Name: state.QName(), Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: 0}
		msg.Answer = append(msg.Answer, &dns.CNAME{Hdr: cnameheader, Target: cnameTarget})
		m.AddARecord(msg, &state, mdnsHosts, cnameTarget)
		log.Debug(msg)
		w.WriteMsg(msg)
		return dns.RcodeSuccess, nil
	}

	srvEntry, present := srvHosts[state.Name()]
	if present {
		srvheader := dns.RR_Header{Name: state.QName(), Rrtype: dns.TypeSRV, Class: dns.ClassINET, Ttl: 0}
		for _, host := range srvEntry {
			msg.Answer = append(msg.Answer, &dns.SRV{Hdr: srvheader, Target: host.Host, Priority: 0, Weight: 10, Port: uint16(host.Port)})
		}
		log.Debug(msg)
		w.WriteMsg(msg)
		return dns.RcodeSuccess, nil
	}
	return plugin.NextOrFailure(m.Name(), m.Next, ctx, w, r)
}

func (m *MDNS) BrowseMDNS() {
	entriesCh := make(chan *mdns.ServiceEntry, 4)
	srvEntriesCh := make(chan *mdns.ServiceEntry, 4)
	m.mutex.Lock()
	defer m.mutex.Unlock()
	// Clear maps so we don't have stale entries and so we don't append infinitely
	// to the srvHosts map
	for k := range *m.mdnsHosts {
		delete(*m.mdnsHosts, k)
	}
	for k := range *m.srvHosts {
		delete(*m.srvHosts, k)
	}
	go func() {
		log.Debug("Retrieving mDNS entries")
		for entry := range entriesCh {
			log.Debugf("Name: %s, Host: %s, AddrV4: %s, AddrV6: %s\n", entry.Name, entry.Host, entry.AddrV4, entry.AddrV6)
			// Hacky - coerce .local to our domain
			// I was having trouble using domains other than .local. Need further investigation.
			// After further investigation, maybe this is working as intended:
			// https://lists.freedesktop.org/archives/avahi/2006-February/000517.html
			hostCustomDomain := m.ReplaceLocal(entry.Host)
			(*m.mdnsHosts)[hostCustomDomain] = entry
		}
	}()

	go func() {
		log.Debug("Retrieving SRV mDNS entries")
		for entry := range srvEntriesCh {
			log.Debugf("Name: %s, Host: %s, AddrV4: %s, AddrV6: %s\n", entry.Name, entry.Host, entry.AddrV4, entry.AddrV6)
			hostCustomDomain := m.ReplaceLocal(entry.Host)
			srvName := strings.SplitN(m.ReplaceLocal(entry.Name), ".", 2)[1]
			entry.Host = hostCustomDomain
			(*m.srvHosts)[srvName] = append((*m.srvHosts)[srvName], entry)
		}
	}()

	mdns.Lookup("_workstation._tcp", entriesCh)
	close(entriesCh)
	mdns.Lookup("_etcd-server-ssl._tcp", srvEntriesCh)
	close(srvEntriesCh)
	log.Debug(m.mdnsHosts)
	log.Debug(m.srvHosts)
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
