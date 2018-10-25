CURRENT_DIR=$(shell pwd)
IMAGE_NAME=dcos/dcos-ui-update-service
DOCKER_DIR=/src

.PHONY: start 
start: docker-image
	$(call inDocker,rerun -v --config=rerun.json)

.PHONY: watchTest 
watchTest: docker-image 
	$(call inDocker,rerun -v --test)

.PHONY: test
test: lint
	$(call inDocker,go test -race -cover ./...)

.PHONY: lint
lint: docker-image
	$(call inDocker,go build ./ && gometalinter --config=.gometalinter.json ./...)

.PHONY: docker-image
docker-image:
ifndef NO_DOCKER
	docker build -t $(IMAGE_NAME) -f Dockerfile.dev .
endif

.PHONY: build
build: docker-image
	$(call inDocker,env GOOS=linux GO111MODULE=on go build \
		-o build/dcos-ui-update-service ./)

.PHONY: clean
clean:
	rm -rf build

ifdef NO_DOCKER
  define inDocker
    $1
  endef
else
  define inDocker
    docker run -p 5000:5000/tcp \
      -e CLUSTER_URL=$(CLUSTER_URL) \
      -v $(CURRENT_DIR):$(DOCKER_DIR) \
      -it \
      --name dcos-ui-service \
      --rm \
      $(IMAGE_NAME) \
    /bin/sh -c "$1"
  endef
endif