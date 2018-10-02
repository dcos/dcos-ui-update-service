CURRENT_DIR=$(shell pwd)
IMAGE_NAME=dcos/dcos-ui-update-service
DOCKER_DIR=/src

.PHONY: start 
start: docker-image
	$(call inDocker,rerun -v)

.PHONY: watchTest 
watchTest: docker-image 
	$(call inDocker,rerun -v --test)

.PHONY: test
test: vet
	$(call inDocker,go test -race -cover ./...)

.PHONY: vet
vet: lint
	$(call inDocker,go vet ./...)

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
      -p 8080:80 \
      -v $(CURRENT_DIR):$(DOCKER_DIR) \
      -it \
      --rm \
      $(IMAGE_NAME) \
    /bin/sh -c "$1"
  endef
endif