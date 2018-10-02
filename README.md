# DC/OS UI Update Service

Requested by Daniel Schmidt

## Development

### With docker

The easiest way to develop on this project is to use the make file (which relies on docker).
For example `make test` will run linting and tests and it will run all these commands inside a docker container.

### Inside docker

You can use this command to start an interactive docker shell with everything you need preinstalled:

`docker build -t dev -f Dockerfile.dev . && docker run -p 8080:80 -v $(pwd):/src -it dev /bin/bash`

## Production Deployment

In the future we will push this image to dockerhub automatically.
Currently to build a production docker image you need to run `docker build .`
