FROM gliderlabs/alpine:3.3
ENTRYPOINT ["/bin/resolvable"]

RUN apk add --no-cache -t build-deps go git mercurial
COPY ./config /config
COPY . /src
RUN cd /src && ./build.sh "$(cat VERSION)"

ONBUILD COPY ./modules.go /src/modules.go
ONBUILD RUN cd /src && ./build.sh "$(cat VERSION)-custom"
