package hello

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/coredns/coredns/plugin"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/request"

	"github.com/hashicorp/mdns"
	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

var log = clog.NewWithPlugin("hello")

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


type Hello struct {
	Next plugin.Handler
	Domain string
}

func (h Hello) ReplaceLocal(input string) (string) {
	// Replace .local domain with our configured custom domain
	fqDomain := "." + h.Domain + "."
	return input[0:len(input) - 7] + fqDomain
}

func (h Hello) AddARecord(msg *dns.Msg, state *request.Request, hosts map[string]*mdns.ServiceEntry, name string) bool {
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

func (h Hello) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {

	log.Debug("Received query")
	fmt.Println("Hello world!")
	msg := new(dns.Msg)
	msg.SetReply(r)
	state := request.Request{W: w, Req: r}

	if state.QType() != dns.TypeA && state.QType() != dns.TypeAAAA && state.QType() != dns.TypeSRV && state.QType() != dns.TypeCNAME {
		fmt.Println("Bailing")
		return plugin.NextOrFailure(h.Name(), h.Next, ctx, w, r)
	}

	// MDNS browsing
	entriesCh := make(chan *mdns.ServiceEntry, 4)
	srvEntriesCh := make(chan *mdns.ServiceEntry, 4)
	mdnsHosts := make(map[string]*mdns.ServiceEntry)
	srvHosts := make(map[string][]*mdns.ServiceEntry)
	go func() {
		fmt.Println("Running")
		for entry := range entriesCh {
			fmt.Printf("Name: %s, Host: %s, AddrV4: %s, AddrV6: %s\n", entry.Name, entry.Host, entry.AddrV4, entry.AddrV6)
			// Hacky - coerce .local to our domain
			// I was having trouble using domains other than .local. Need further investigation.
			// After further investigation, maybe this is working as intended:
			// https://lists.freedesktop.org/archives/avahi/2006-February/000517.html
			hostCustomDomain := h.ReplaceLocal(entry.Host)
			mdnsHosts[hostCustomDomain] = entry
		}
	}()

	go func() {
		fmt.Println("Running SRV")
		for entry := range srvEntriesCh {
			fmt.Printf("Name: %s, Host: %s, AddrV4: %s, AddrV6: %s\n", entry.Name, entry.Host, entry.AddrV4, entry.AddrV6)
			hostCustomDomain := h.ReplaceLocal(entry.Host)
			srvName := strings.SplitN(h.ReplaceLocal(entry.Name), ".", 2)[1]
			entry.Host = hostCustomDomain
			srvHosts[srvName] = append(srvHosts[srvName], entry)
		}
	}()

	mdns.Lookup("_workstation._tcp", entriesCh)
	mdns.Lookup("_etcd-server-ssl._tcp", srvEntriesCh)
	close(entriesCh)
	fmt.Println(mdnsHosts)
	fmt.Println(srvHosts)

	msg.Answer = []dns.RR{}

	if h.AddARecord(msg, &state, mdnsHosts, state.Name()) {
		fmt.Println(msg)
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
				cname := "etcd-" + strconv.Itoa(i) + "." + h.Domain + "."
				cnames[cname] = host.Host
			}
		}
	}
	fmt.Println(cnames)
	cnameTarget, present := cnames[state.Name()]
	if present {
		cnameheader := dns.RR_Header{Name: state.QName(), Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: 60}
		msg.Answer = append(msg.Answer, &dns.CNAME{Hdr: cnameheader, Target: cnameTarget})
		h.AddARecord(msg, &state, mdnsHosts, cnameTarget)
		fmt.Println(msg)
		w.WriteMsg(msg)
		return dns.RcodeSuccess, nil
	}

	srvEntry, present := srvHosts[state.Name()]
	if present {
		srvheader := dns.RR_Header{Name: state.QName(), Rrtype: dns.TypeSRV, Class: dns.ClassINET, Ttl: 60}
		for _, host := range srvEntry {
			msg.Answer = append(msg.Answer, &dns.SRV{Hdr: srvheader, Target: host.Host, Priority: 0, Weight: 10, Port: uint16(host.Port)})
		}
		fmt.Println(msg)
		w.WriteMsg(msg)
		return dns.RcodeSuccess, nil
	}
	return plugin.NextOrFailure(h.Name(), h.Next, ctx, w, r)
}

func (h Hello) Name() string { return "hello" }

type ResponsePrinter struct {
	dns.ResponseWriter
}

func NewResponsePrinter(w dns.ResponseWriter) *ResponsePrinter {
	return &ResponsePrinter{ResponseWriter: w}
}

func (r *ResponsePrinter) WriteMsg(res *dns.Msg) error {
	fmt.Fprintln(out, h)
	return r.ResponseWriter.WriteMsg(res)
}

var out io.Writer = os.Stdout

const h = "hello"
