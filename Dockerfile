FROM gliderlabs/alpine:3.1
ENTRYPOINT ["/bin/resolve"]

COPY . /go/src/github.com/mgood/resolve
RUN apk-install -t build-deps go git mercurial \
	&& cd /go/src/github.com/mgood/resolve \
	&& export GOPATH=/go \
	&& go get \
	&& go build -ldflags "-X main.Version $(cat VERSION)" -o /bin/resolve \
	&& rm -rf /go \
	&& apk del --purge build-deps
