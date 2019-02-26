package main

import (
	_ "github.com/coredns/coredns/core/plugin"
	_ "github.com/metalkube/coredns-mdns/pkg/mdns"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/coremain"
)

// We need to inject our plugin after the cache plugin for caching to work
func findCacheIndex() int {
	for i, value := range dnsserver.Directives {
		if value == "cache" {
			return i
		}
	}
	return -1
}

var cachePos = findCacheIndex()
var directives = append(dnsserver.Directives[:cachePos + 1], append([]string{"mdns"}, dnsserver.Directives[cachePos + 1:]...)...)

func init() {
	dnsserver.Directives = directives
}

func main() {
	coremain.Run()
}
