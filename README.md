# docs-ui-update-service

Requested by Daniel Schmidt

## Development

We recommend development within a docker container for an easy setup.
For this please use a command like `docker build -t dev -f Dockerfile.dev . && docker run -v $(pwd):/src -it dev /bin/bash`

## Production Deployment

In the future we will push this image to dockerhub automatically.
Currently to build a production docker image you need to run `docker build .`
