package main

import (
	_ "github.com/coredns/coredns/core/plugin"
	_ "github.com/metalkube/coredns-mdns/pkg/mdns"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/coremain"
)

var directives = append([]string{"mdns"}, dnsserver.Directives...)

func init() {
	dnsserver.Directives = directives
}

func main() {
	coremain.Run()
}
