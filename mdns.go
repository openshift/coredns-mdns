package mdns

import (
	"fmt"
	"io"
	"os"
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

type MDNS struct {
	Next   plugin.Handler
	Domain string
	mutex *sync.RWMutex
	mdnsHosts *map[string]*mdns.ServiceEntry
	srvHosts *map[string][]*mdns.ServiceEntry
	cnames *map[string]string
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
		aheader := dns.RR_Header{Name: name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60}
		msg.Answer = append(msg.Answer, &dns.A{Hdr: aheader, A: answerEntry.AddrV4})
		aaaaheader := dns.RR_Header{Name: name, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 60}
		msg.Answer = append(msg.Answer, &dns.AAAA{Hdr: aaaaheader, AAAA: answerEntry.AddrV6})
		return true
	}
	return false
}

// Return the node index from a hostname.
// For example, the return value from "master-0.ostest.test.metalkube.org" would be "0"
func GetIndex(host string) string {
	shortname := strings.Split(host, ".")[0]
	return shortname[strings.LastIndex(shortname, "-")+1:]
}

func (m MDNS) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {

	log.Debug("Received query")
	msg := new(dns.Msg)
	msg.SetReply(r)
	state := request.Request{W: w, Req: r}
	// Just for convenience so we don't have to keep dereferencing these
	mdnsHosts := *m.mdnsHosts
	srvHosts := *m.srvHosts
	cnames := *m.cnames

	if !strings.HasSuffix(state.QName(), m.Domain + ".") {
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
	mdnsHosts := make(map[string]*mdns.ServiceEntry)
	srvHosts := make(map[string][]*mdns.ServiceEntry)
	cnames := make(map[string]string)
	go func() {
		log.Debug("Retrieving mDNS entries")
		for entry := range entriesCh {
			// Make a copy of the entry so mdns can't later overwrite our changes
			localEntry := *entry
			log.Debugf("Name: %s, Host: %s, AddrV4: %s, AddrV6: %s\n", localEntry.Name, localEntry.Host, localEntry.AddrV4, localEntry.AddrV6)
			// Hacky - coerce .local to our domain
			// I was having trouble using domains other than .local. Need further investigation.
			// After further investigation, maybe this is working as intended:
			// https://lists.freedesktop.org/archives/avahi/2006-February/000517.html
			hostCustomDomain := m.ReplaceLocal(localEntry.Host)
			mdnsHosts[hostCustomDomain] = entry
		}
	}()

	go func() {
		log.Debug("Retrieving SRV mDNS entries")
		for entry := range srvEntriesCh {
			// Make a copy of the entry so mdns can't later overwrite our changes
			localEntry := *entry
			log.Debugf("Name: %s, Host: %s, AddrV4: %s, AddrV6: %s\n", localEntry.Name, localEntry.Host, localEntry.AddrV4, localEntry.AddrV6)
			hostCustomDomain := m.ReplaceLocal(localEntry.Host)
			srvName := strings.SplitN(m.ReplaceLocal(localEntry.Name), ".", 2)[1]
			cname := "etcd-" + GetIndex(localEntry.Host) + "." + m.Domain + "."
			localEntry.Host = cname
			cnames[cname] = hostCustomDomain
			srvHosts[srvName] = append(srvHosts[srvName], &localEntry)
		}
	}()

	mdns.Lookup("_workstation._tcp", entriesCh)
	close(entriesCh)
	mdns.Lookup("_etcd-server-ssl._tcp", srvEntriesCh)
	close(srvEntriesCh)
	m.mutex.Lock()
	defer m.mutex.Unlock()
	// Clear maps so we don't have stale entries
	for k := range *m.mdnsHosts {
		delete(*m.mdnsHosts, k)
	}
	for k := range *m.srvHosts {
		delete(*m.srvHosts, k)
	}
	for k := range *m.cnames {
		delete(*m.cnames, k)
	}
	// Copy values into the shared maps only after we've collected all of them.
	// This prevents us from having to lock during the entire mdns browse time.
	for k, v := range mdnsHosts {
		(*m.mdnsHosts)[k] = v
	}
	for k, v := range srvHosts {
		(*m.srvHosts)[k] = v
	}
	for k, v := range cnames {
		(*m.cnames)[k] = v
	}
	log.Debugf("mdnsHosts: %v", m.mdnsHosts)
	log.Debugf("srvHosts: %v", m.srvHosts)
	log.Debugf("cnames: %v", m.cnames)
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
