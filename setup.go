package mdns

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	clog "github.com/coredns/coredns/plugin/pkg/log"

	"github.com/whyrusleeping/mdns"
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
	minSRV := 3
	if c.NextArg() {
		val, err := strconv.Atoi(c.Val())
		if err != nil {
			text := fmt.Sprintf("Invalid minSRV: %s", err)
			return plugin.Error("mdns", errors.New(text))
		}
		minSRV = val
	}
	if c.NextArg() {
		return plugin.Error("mdns", c.ArgErr())
	}

	// Because the plugin interface uses a value receiver, we need to make these
	// pointers so all copies of the plugin point at the same maps.
	mdnsHosts := make(map[string]*mdns.ServiceEntry)
	srvHosts := make(map[string][]*mdns.ServiceEntry)
	cnames := make(map[string]string)
	mutex := sync.RWMutex{}
	m := MDNS{Domain: strings.TrimSuffix(domain, "."), minSRV: minSRV, mutex: &mutex, mdnsHosts: &mdnsHosts, srvHosts: &srvHosts, cnames: &cnames}

	c.OnStartup(func() error {
		// mdns is quite noisy, and we don't really care about any of its messages
		if ! clog.D {
			mdns.DisableLogging = true
		}
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
