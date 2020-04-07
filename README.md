# mDNS

## Name

mDNS - CoreDNS plugin that reads mDNS records from the local network and responds
to queries based on those records.

## Description

Useful for providing mDNS records to non-mDNS-aware applications by making them
accessible through a standard DNS server.

## Syntax

~~~
mdns
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
