#!/bin/bash

# Clone openshift/coredns to a temporary directory and tell it to
# use our local copy of coredns-mdns when it builds.

# Optionally takes one parameter: the directory in which to clone
# coredns. If not provided, a temporary directory will be created
# and deleted after the build finishes. If it is provided, the
# directory specified will be used and not deleted at the end.

# The resulting coredns binary will be copied to the coredns-mdns
# repo root.

set -ex -o pipefail

export GOPATH="${1:-$(mktemp -d)}"
if [ -z "${1:-}" ]
then
    trap "chmod -R u+w $GOPATH; rm -rf $GOPATH" EXIT
fi
mkdir -p $GOPATH/src/github.com/coredns
source_dir=$(readlink -f "$(dirname "$0")/..")

export COREDNS_REPO="${COREDNS_REPO:-https://github.com/openshift/coredns}"
export COREDNS_BRANCH="${COREDNS_BRANCH:-master}"
cd $GOPATH/src/github.com/coredns
if [ ! -d coredns ]
then
    git clone ${COREDNS_REPO}
fi
cd coredns
git checkout ${COREDNS_BRANCH}
# Make coredns use our local source
rm vendor/github.com/openshift/coredns-mdns/*
cp $source_dir/*.go vendor/github.com/openshift/coredns-mdns
GO111MODULE=on GOFLAGS=-mod=vendor go build -o coredns .
cp coredns "$source_dir"
