FROM alpine:3.2
ENTRYPOINT ["/bin/resolvable"]

COPY ./config /config
COPY . /src
RUN cd /src && ./build.sh "$(cat VERSION)"

ONBUILD COPY ./modules.go /src/modules.go
ONBUILD RUN cd /src && ./build.sh "$(cat VERSION)-custom"
