CURRENT_DIR=$(shell pwd)
IMAGE_NAME=dcos/dcos-ui-update-service
DOCKER_DIR=/src

.PHONY: test
test: lint
	$(call inDocker,go test -race -cover ./...)

.PHONY: lint
lint: docker-image
	$(call inDocker,gometalinter --config=.gometalinter.json ./...)

.PHONY: docker-image
docker-image:
ifndef NO_DOCKER
	docker build -t $(IMAGE_NAME) -f Dockerfile.dev .
endif

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
      -v $(CURRENT_DIR):$(DOCKER_DIR) \
      --rm \
      $(IMAGE_NAME) \
    /bin/sh -c "$1"
  endef
endif