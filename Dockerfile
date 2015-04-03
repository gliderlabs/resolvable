FROM gliderlabs/alpine:3.1
ENTRYPOINT ["/bin/resolvable"]

COPY . /go/src/github.com/mgood/resolvable
RUN apk-install -t build-deps go git mercurial \
	&& cd /go/src/github.com/mgood/resolvable \
	&& export GOPATH=/go \
	&& go get \
	&& go build -ldflags "-X main.Version $(cat VERSION)" -o /bin/resolvable \
	&& rm -rf /go \
	&& apk del --purge build-deps
