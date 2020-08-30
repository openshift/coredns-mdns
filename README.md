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

If `minimum SRV records` is specified in the configuration, the plugin will wait
until it has at least that many SRV records before responding with any of them.
`minimum SRV records` defaults to `3`.

~~~ corefile
example.com {
    mdns example.com 2
}
~~~

This would mean that at least two SRV records of a given type would need to be
present for any SRV records to be returned. If only one record is found, any
requests for that type of SRV record would receive no results.

If `filter text` is specified in the configuration, the plugin will ignore any
mDNS records that do not include the specified text in the service name. This
allows the plugin to be used in environments where there may be mDNS services
advertised that are not intended for use with it. When `filter text` is not
set, all records will be processed.

~~~ corefile
example.com {
    mdns example.com 3 my-id
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
    mdns example.com 3 "" 192.168.1.1
}
~~~

This configuration will only send multicast packets to the interface assigned
the `192.168.1.1` address. The interface lookup is dynamic each time an mDNS
query is sent, so if the address moves to a different interface the plugin
will automatically switch to the new one.

## Service Discovery

Queries to mDNS are constrained to the `.local` TLD, alternative domains that an mDNS server publishes are not supported by this plugin.

If a query is not responding, check the log for hosts discovered by this plugin reported as `mdnsHosts`. The plugin will populate `mdnsHosts` by **only discovering** mDNS services of the type `_workstation._tcp`.

### Publishing `_workstation._tcp` service with Avahi

Avahi is commonly installed on Linux systems as the default mDNS server. Your distro may have it configured to publish this service by default, however distros that follow upstream have this feature disabled for security reasons. While it is not required to be enabled to respond to explicit requests, it is required for service discovery over mDNS which this plugin relies on.

You can publish the workstation service by enabling `publish-workstation=yes` under `[publish]` within your Avahi config (`/etc/avahi/avahi-daemon.conf`). To check that it's discovered, use `avahi-browse --all --resolve --terminate`.

You can alternatively publish a service to an existing hostname (eg configured in `/etc/avahi/hosts` or `avahi-publish --address <hostname> <ip>`) with `avahi-publish --service friendly_name _workstation._tcp 42 --domain=local --host=your-hostname.local`, `--domain` allows for publishing to a different domain(group, defaults to `local`), while `--host` is the FQDN including the `.local` (this can differ from `.local`, as the plugin discovery is actually based on the `local` domain, however the plugin does not properly handle the FQDN), the port number has no relevance to this plugins functionality but is required by `avahi-publish --service`.
