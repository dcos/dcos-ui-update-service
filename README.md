# DC/OS UI Update Service

Requested by Daniel Schmidt

## Development

### With docker

The easiest way to develop on this project is to use the make file (which relies on docker).
For example `make test` will run linting and tests and it will run all these commands inside a docker container.

### Inside docker

You can use this command to start an interactive docker shell with everything you need preinstalled:

`docker build -t dev -f Dockerfile.dev . && docker run -v $(pwd):/src -it dev /bin/bash`

You can run the service inside docker by exporting `$CLUSTER_URL` and running `make start`

```bash
$ export CLUSTER_URL=<path_to_a_cluster>
$ make start
```

This will run the service inside docker with [rerun](https://github.com/ivpusic/rerun) that watches for changes and restarts the app when files are saved. By default running in docker will start the service on port 5000 of your local machine. The rerun arguments can be found in `rerun.json`.

## Production Deployment

In the future we will push this image to dockerhub automatically.
Currently to build a production docker image you need to run `docker build .`
