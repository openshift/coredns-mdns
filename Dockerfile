FROM registry.svc.ci.openshift.org/openshift/release:golang-1.10 AS builder
WORKDIR /go/src/github.com/metalkube/coredns-mdns
COPY . .
RUN go get -v github.com/coredns/coredns github.com/hashicorp/mdns golang.org/x/net/context
RUN go build -o coredns .

FROM registry.svc.ci.openshift.org/openshift/origin-v4.0:base
COPY --from=builder /go/src/github.com/metalkube/coredns-mdns/coredns /usr/bin/

ENTRYPOINT ["/usr/bin/coredns"]

LABEL io.k8s.display-name="CoreDNS" \
      io.k8s.description="CoreDNS delivers the DNS and Discovery Service for a Kubernetes cluster." \
      maintainer="Antoni Segura Puimedon <antoni@redhat.com>"
