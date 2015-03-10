FROM gliderlabs/alpine:3.1
ENTRYPOINT ["/bin/docker-resolver"]

COPY . /go/src/github.com/mgood/docker-resolver
RUN apk-install -t dnsmasq build-deps \
	&& cd /go/src/github.com/mgood/docker-resolver \
	&& export GOPATH=/go \
	&& go get \
	# && go build -ldflags "-X main.Version $(cat VERSION)" -o /bin/docker-resolver \
	&& go build -o /bin/docker-resolver \
	&& rm -rf /go \
	&& apk del --purge build-deps
