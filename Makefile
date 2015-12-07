NAME=resolvable
VERSION=$(shell cat VERSION)

dev:
	#@docker history $(NAME):dev &> /dev/null \
	#	|| docker build -f Dockerfile.dev -t $(NAME):dev .
	docker build -f Dockerfile.dev -t $(NAME):dev .
	@docker run --rm \
		--hostname $(NAME) \
		-v /var/run/docker.sock:/tmp/docker.sock \
		-v /etc/resolvconf/resolv.conf.d/head:/tmp/resolv.conf \
		-v /var/run/dbus/system_bus_socket:/var/run/dbus/system_bus_socket \
		$(NAME):dev

devupstart:
	docker build -f Dockerfile.dev -t $(NAME):dev .
	@docker run --rm -it \
		--hostname $(NAME) \
		--privileged \
		-v /var/run/docker.sock:/tmp/docker.sock \
		-v /etc/resolvconf/resolv.conf.d/head:/tmp/resolv.conf \
		-v /var/run/dbus/system_bus_socket:/var/run/dbus/system_bus_socket \
		$(NAME):dev /bin/ash

build:
	mkdir -p build
	docker build -t $(NAME):$(VERSION) .
	docker save $(NAME):$(VERSION) | gzip -9 > build/$(NAME)_$(VERSION).tgz

test:
	GOMAXPROCS=4 go test -v ./... -race

release:
	rm -rf release && mkdir release
	go get github.com/progrium/gh-release/...
	cp build/* release
	gh-release create gliderlabs/$(NAME) $(VERSION) \
		$(shell git rev-parse --abbrev-ref HEAD) $(VERSION)

circleci:
	rm ~/.gitconfig
ifneq ($(CIRCLE_BRANCH), release)
	echo build-$$CIRCLE_BUILD_NUM > VERSION
endif

.PHONY: build release
