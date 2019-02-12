package hello

import (
	"fmt"
	"io"
	"os"
    "net"

	"github.com/coredns/coredns/plugin"
    "github.com/coredns/coredns/request"
	//"github.com/coredns/coredns/plugin/metrics"
	clog "github.com/coredns/coredns/plugin/pkg/log"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

var log = clog.NewWithPlugin("hello")

type Hello struct {
    Next plugin.Handler
}

func (h Hello) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
    log.Debug("Received query")
    fmt.Println("Hello world!")
    //pw := NewResponsePrinter(w)
    msg := new(dns.Msg)
    msg.SetReply(r)
    state := request.Request{W: w, Req: r}

    header := dns.RR_Header{Name: state.QName(), Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: 0}
    msg.Answer = []dns.RR{&dns.CNAME{Hdr: header, Target: "hello.world."}}
    aheader := dns.RR_Header{Name: "hello.world.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 0}
    msg.Answer = append(msg.Answer, &dns.A{Hdr: aheader, A: net.ParseIP("1.2.3.4")})

    //header := dns.RR_Header{Name: state.QName(), Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 0}
    //msg.Answer = []dns.RR{&dns.A{Hdr: header, A: net.ParseIP("1.2.3.4")}}

    fmt.Println(msg)
    w.WriteMsg(msg)
    return dns.RcodeSuccess, nil
    //return plugin.NextOrFailure(h.Name(), h.Next, ctx, pw, r)
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
