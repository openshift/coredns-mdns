package hello

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/coredns/coredns/plugin"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/request"

	"github.com/hashicorp/mdns"
	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

var log = clog.NewWithPlugin("hello")

type Hello struct {
	Next plugin.Handler
	Domain string
	Cluster string
}

func (h Hello) ReplaceLocal(input string) (string) {
	// Replace .local domain with our configured custom domain
	fqDomain := "." + h.Domain + "."
	return input[0:len(input) - 7] + fqDomain
}

func (h Hello) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {

	log.Debug("Received query")
	fmt.Println("Hello world!")
	msg := new(dns.Msg)
	msg.SetReply(r)
	state := request.Request{W: w, Req: r}

	if state.QType() != dns.TypeA && state.QType() != dns.TypeAAAA && state.QType() != dns.TypeSRV {
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
			fmt.Println(hostCustomDomain)
			mdnsHosts[hostCustomDomain] = entry
		}
	}()

	go func() {
		fmt.Println("Running SRV")
		for entry := range srvEntriesCh {
			fmt.Printf("Name: %s, Host: %s, AddrV4: %s, AddrV6: %s\n", entry.Name, entry.Host, entry.AddrV4, entry.AddrV6)
			hostCustomDomain := h.ReplaceLocal(entry.Host)
			hostCustomDomain = hostCustomDomain[0:len(hostCustomDomain) - 1]
			srvName := strings.SplitN(h.ReplaceLocal(entry.Name), ".", 2)[1]
			srvName = "_etcd-server-ssl._tcp.test.fooxample.com."
			fmt.Println(srvName)
			fmt.Println(hostCustomDomain)
			entry.Host = hostCustomDomain
			srvHosts[srvName] = append(srvHosts[srvName], entry)
		}
	}()

	mdns.Lookup("_workstation._tcp", entriesCh)
	mdns.Lookup("_etcd-server-ssl._tcp", srvEntriesCh)
	close(entriesCh)
	fmt.Println(mdnsHosts)
	fmt.Println(srvHosts)

	answerEntry, present := mdnsHosts[state.Name()]
	if present {
		aheader := dns.RR_Header{Name: state.QName(), Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60}
		msg.Answer = []dns.RR{&dns.A{Hdr: aheader, A: answerEntry.AddrV4}}
		aaaaheader := dns.RR_Header{Name: state.QName(), Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 60}
		msg.Answer = append(msg.Answer, &dns.AAAA{Hdr: aaaaheader, AAAA: answerEntry.AddrV6})
		fmt.Println(msg)
		w.WriteMsg(msg)
		return dns.RcodeSuccess, nil
	}
	srvEntry, present := srvHosts[state.Name()]
	if present {
		srvheader := dns.RR_Header{Name: state.QName(), Rrtype: dns.TypeSRV, Class: dns.ClassINET, Ttl: 60}
		msg.Answer = []dns.RR{}
		for _, host := range srvEntry {
			msg.Answer = append(msg.Answer, &dns.SRV{Hdr: srvheader, Target: host.Host, Priority: 0, Weight: 10, Port: 2380})
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
