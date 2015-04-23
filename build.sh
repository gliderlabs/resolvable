#!/bin/sh
set -e
apk add --update -t build-deps go git mercurial
mkdir -p /go/src/github.com/mgood
cp -r /src /go/src/github.com/mgood/resolvable
cd /go/src/github.com/mgood/resolvable
export GOPATH=/go
go get
go build -ldflags "-X main.Version $1" -o /bin/resolvable
apk del --purge build-deps
rm -rf /go
rm -rf /var/cache/apk/*
