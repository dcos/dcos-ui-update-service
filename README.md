# DC/OS UI Update Service

Requested by Daniel Schmidt

## Development

### With docker

The easiest way to develop on this project is to use the make file (which relies on docker).
For example `make test` will run linting and tests and it will run all these commands inside a docker container.

### Inside docker

You can run the service inside docker by exporting `$CLUSTER_URL`, `$AUTH_TOKEN` and running `make start`

```bash
$ export CLUSTER_URL=<path_to_a_cluster>
$ export AUTH_TOKEN=<token>
$ make start
```

This will run the service inside docker with [rerun](https://github.com/ivpusic/rerun) that watches for changes and restarts the app when files are saved. By default running in docker will start the service on port 5000 and serve on the same port of your local machine. The rerun arguments can be found in `rerun.json`. We also run a small proxy server to make authenticated calls to cosmos running in your cluster.

To get an `AUTH_TOKEN` make a login call to your cluster:
```bash
$ export CLUSTER_URL=<path_to_a_cluster>
$ curl -X "POST" "$CLUSTER_URL/acs/api/v1/auth/login" \
     -H 'Content-Type: application/json; charset=utf-8' \
     -d $'{
  "uid": "<user_name>",
  "password": "<password>"
}' --insecure
{
  "token": "<AUTH_TOKEN>"
}
$ export AUTH_TOKEN=<AUTH_TOKEN>
```
Be sure to replace `<user_name>` and `<password>` with the correct credentials for the cluster you are testing with. If the login curl command fails double-check if the `CLUSTER_URL` ends with a `/` and update either it or the url in the curl command accordingly.

## Production Deployment

In the future we will push this image to dockerhub automatically.
Currently to build a production docker image you need to run `docker build .`
