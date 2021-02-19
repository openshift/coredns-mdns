# mDNS

## Name

mDNS - CoreDNS plugin that reads mDNS records from the local network and responds
to queries based on those records.

## Description

Useful for providing mDNS records to non-mDNS-aware applications by making them
accessible through a standard DNS server.

## Syntax

~~~
mdns example.com [minimum SRV records] [filter text] [bind address]
~~~

## Examples

As a prerequisite to using this plugin, there must be systems on the local
network broadcasting mDNS records. Note that the .local domain will be
replaced with the configured domain. For example, `test.local` would become
`test.example.com` using the configuration below.

Specify the domain for the records.

~~~ corefile
example.com {
	mdns example.com
}
~~~

And test with `dig`:

~~~ txt
dig @localhost baremetal-test-extra-1.example.com

;; ANSWER SECTION:
baremetal-test-extra-1.example.com. 60 IN A   12.0.0.24
baremetal-test-extra-1.example.com. 60 IN AAAA fe80::f816:3eff:fe49:19b3
~~~

The `minimum SRV records` parameter was only used by a removed feature. It
has no effect, but for backward compatibility it must be present to use any
subsequent parameters.

~~~ corefile
example.com {
    mdns example.com 0
}
~~~

If `filter text` is specified in the configuration, the plugin will ignore any
mDNS records that do not include the specified text in the service name. This
allows the plugin to be used in environments where there may be mDNS services
advertised that are not intended for use with it. When `filter text` is not
set, all records will be processed.

~~~ corefile
example.com {
    mdns example.com 0 my-id
}
~~~

This configuration would ignore any mDNS records that do not contain the
string "my-id" in their service name.

If `bind address` is specified in the configuration, the plugin will only send
mDNS traffic to the associated interface. This prevents sending multicast
packets on interfaces where that may not be desirable. To use `bind address`
without setting a filter, set `filter text` to "".

~~~ corefile
example.com {
    mdns example.com 0 "" 192.168.1.1
}
~~~

This configuration will only send multicast packets to the interface assigned
the `192.168.1.1` address. The interface lookup is dynamic each time an mDNS
query is sent, so if the address moves to a different interface the plugin
will automatically switch to the new one.

## Service Discovery

Queries to mDNS are constrained to the `local` domain, alternative domains that an mDNS server publishes are not supported by this plugin. This does not restrict you to `.local` TLD, as the address is referenced from the service responses belonging to the `local` domain.

If you have multiple network interfaces that respond to mDNS for your host(eg on the same system that CoreDNS is running), this can result in the wrong IP address returned for a hostname due to a race condition with multicast. Make sure you configure your mDNS server to whitelist the network interface with the assigned IP address you want to associate the hostname to.

If a query is not responding, check the log for hosts discovered by this plugin reported as `mdnsHosts`. The plugin will populate `mdnsHosts` by **only discovering** mDNS services of the type `_workstation._tcp`.

### Publishing `_workstation._tcp` service with Avahi

Avahi is commonly installed on Linux systems as the default mDNS server. Your distro may have it configured to publish this service by default, however distros that [follow upstream defaults](https://github.com/lathiat/avahi/blob/d1e71b320d96d0f213ecb0885c8313039a09f693/avahi-daemon/avahi-daemon.conf#L50) have this feature disabled for security reasons. While it is not required to be enabled to respond to explicit requests, it is required for service discovery over mDNS which this plugin relies on.

You can list the results `coredns-mdns` will discover with: `avahi-browse --resolve --terminate _workstation._tcp`

If your hostname is missing but can be resolved with `avahi-resolve --name your-hostname-here.local`, it needs to be published as a workstation service.

Edit `/etc/avahi/avahi-daemon.conf`:

```
[publish]
publish-workstation=yes
```

If the hostname is defined by `avahi-publish --address <hostname> <ip>`, `/etc/avahi/hosts`, or other means like D-Bus, you can publish the workstation service to point to that hostname with: 

`avahi-publish --service friendly_name _workstation._tcp 9`

- The last argument is an associated port for the service, it is not important for this plugin but a value is required to publish.
- You can use `--host=<hostname>` to choose a value that differs from the default Avahi `host-name` in `/etc/avahi/avahi-daemon.conf`. This should be an FQDN value, the TLD should be appended to it (eg, `--host=your-hostname-here.local`)
- By default it will be published under the `local` domain which `coredns-mdns` searches.
