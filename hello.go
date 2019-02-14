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
}

func (h Hello) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	fqDomain := "." + h.Domain + "."
	log.Debug("Received query")
	fmt.Println("Hello world!")
	msg := new(dns.Msg)
	msg.SetReply(r)
	state := request.Request{W: w, Req: r}

	if state.QType() != dns.TypeA && state.QType() != dns.TypeAAAA {
		fmt.Println("Bailing")
		return plugin.NextOrFailure(h.Name(), h.Next, ctx, w, r)
	}

	// MDNS browsing
	entriesCh := make(chan *mdns.ServiceEntry, 4)
	mdnsHosts := make(map[string]*mdns.ServiceEntry)
	go func() {
		for entry := range entriesCh {
			fmt.Printf("Host: %s, AddrV4: %s, AddrV6: %s\n", entry.Host, entry.AddrV4, entry.AddrV6)
			// Hacky - coerce .local to our domain
			// I was having trouble using domains other than .local. Need further investigation.
			// After further investigation, maybe this is working as intended:
			// https://lists.freedesktop.org/archives/avahi/2006-February/000517.html
			hostCustomDomain := strings.Replace(entry.Host, ".local.", fqDomain, 1)
			fmt.Println(hostCustomDomain)
			mdnsHosts[hostCustomDomain] = entry
		}
	}()

	mdns.Lookup("_workstation._tcp", entriesCh)
	close(entriesCh)
	fmt.Println(mdnsHosts)

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
