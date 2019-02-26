package main

import (
	_ "github.com/coredns/coredns/core/plugin"
	_ "github.com/metalkube/coredns-mdns/pkg/mdns"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/coremain"
)

// We need to inject our plugin after the cache plugin for caching to work
func findCache() int {
	for i, value := range dnsserver.Directives {
		if value == "cache" {
			return i + 1
		}
	}
	return -1
}

var cachePos = findCache()
var directives = append(dnsserver.Directives[:cachePos], append([]string{"mdns"}, dnsserver.Directives[cachePos:]...)...)

func init() {
	dnsserver.Directives = directives
}

func main() {
	coremain.Run()
}
