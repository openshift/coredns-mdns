package mdns

import (
	"time"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"

	"github.com/hashicorp/mdns"
	"github.com/mholt/caddy"
)

func init() {
	caddy.RegisterPlugin("mdns", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	c.Next()
	c.NextArg()
	domain := c.Val()
	if c.NextArg() {
		return plugin.Error("mdns", c.ArgErr())
	}

	// Because the plugin interface uses a value receiver, we need to make these
	// pointers so all copies of the plugin point at the same maps.
	mdnsHosts := make(map[string]*mdns.ServiceEntry)
	srvHosts := make(map[string][]*mdns.ServiceEntry)
	m := MDNS{Domain: domain, mdnsHosts: &mdnsHosts, srvHosts: &srvHosts}

	c.OnStartup(func() error {
		go browseLoop(&m)
		return nil
	})

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		m.Next = next
		return m
	})

	return nil
}

func browseLoop(m *MDNS) {
	for {
		m.BrowseMDNS()
		// 5 seconds seems to be the minimum ttl that the cache plugin will allow
		// Since each browse operation takes around 2 seconds, this should be fine
		time.Sleep(5 * time.Second)
	}
}
