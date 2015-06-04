FROM gliderlabs/alpine:3.1

ENV GOPATH /go
RUN apk-install go git mercurial
COPY . /go/src/github.com/gliderlabs/resolvable
WORKDIR /go/src/github.com/gliderlabs/resolvable
RUN go get
CMD go get \
	&& go build -ldflags "-X main.Version dev" -o /bin/resolvable \
	&& exec /bin/resolvable
