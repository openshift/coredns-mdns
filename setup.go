package hello

import (
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	//"github.com/coredns/coredns/plugin/metrics"

	"github.com/mholt/caddy"
)

func init() {
	caddy.RegisterPlugin("hello", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	c.Next()
	if c.NextArg() {
		return plugin.Error("hello", c.ArgErr())
	}

	c.OnStartup(func() error {
		//once.Do(func() { metrics.MustRegister(c, requestCount) })
		return nil
	})

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		return Hello{Next: next}
	})

	return nil
}
