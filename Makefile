
# detect what platform we're running in so we can use proper command flavors
OS := $(shell uname -s)
ifeq ($(OS),Linux)
sSHA1 := sha1sum
endif
ifeq ($(OS),Darwin)
SHA1 := shasum -a1
endif

DOCKERFILE_DEV_SHA := $(shell cat Dockerfile.dev go.mod | $(SHA1) | awk '{ print $$1 }')

.PHONY: watchTest 
watchTest: docker.build.dev
	$(call inDocker,rerun -v --test)

.PHONY: test
test: lint
	$(call inDocker,go test -race -cover ./...)

.PHONY: lint
lint: docker.build.dev
	$(call inDocker,env GOOS=linux GO111MODULE=on go build ./ && gometalinter --config=.gometalinter.json ./...)

.PHONY: docker.build.dev
docker.build.dev: .docker.build.dev.$(DOCKERFILE_DEV_SHA)

.docker.build.dev.$(DOCKERFILE_DEV_SHA):
	@$(RM) .docker.build.dev.*
	@docker build \
			-t dcos/dcos-ui-update-service-dev:$(DOCKERFILE_DEV_SHA) \
			-f Dockerfile.dev .
	@touch $@

.PHONY: build
build: docker.build.dev
	$(call inDocker,env GOOS=linux GO111MODULE=on go build \
		-o builds/dcos-ui-update-service ./)

.PHONY: clean
clean:
	rm -rf build

ifdef NO_DOCKER
  define inDocker
    $1
  endef
else
  define inDocker
    docker run \
      -v $(CURDIR):/src \
      -w /src \
      --rm -it \
      dcos/dcos-ui-update-service-dev:$(DOCKERFILE_DEV_SHA) /bin/sh -c "$1"
  endef
endif