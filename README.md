# mDNS

## Name

mDNS - CoreDNS plugin that reads mDNS records from the local network and responds
to queries based on those records.

## Description

Useful for providing mDNS records to non-mDNS-aware applications by making them
accessible through a standard DNS server.

## Syntax

~~~
mdns [minimum SRV records] [filter text]
~~~

## Examples

As a prerequisite to using this plugin, there must be systems on the local
network broadcasting mDNS records. 

Specify the domain for the records.

~~~ corefile
.local {
	mdns
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
`minimum SRV records` defaults to `1`.

~~~ corefile
.local {
    mdns 2
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
.local {
    mdns 3 my-id
}
~~~

This configuration would ignore any mDNS records that do not contain the
string "my-id" in their service name.
