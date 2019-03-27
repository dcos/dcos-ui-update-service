# DC/OS UI Update Service

## Configuration

dcos-ui-update service can be configured via json file or command line arguments.

```
Usage:
      --config string
      The path to the optional config file

      --listen-net (default "unix")
      The transport type on which to listen for connections. May be one of 'tcp', 'unix'.

      --listen-addr (default "/run/dcos/dcos-ui-update-service.sock")
      The network address at which to listen for connections.

      --universe-url (default "http://127.0.0.1:7070")
      The URL where universe can be reached.

      --default-ui-path (default "/opt/mesosphere/active/dcos-ui/usr")
      The filesystem path with the default ui distribution (pre-bundled ui).

      --ui-dist-symlink (default "/opt/mesosphere/active/dcos-ui-dist")
      The filesystem symlink path where the ui distributed files are served from.

      --ui-dist-stage-symlink (default "/opt/mesosphere/active/new-dcos-ui-dist")
      The temporary filesystem symlink path that links to where the ui distribution files are located.

      --versions-root (default "/opt/mesosphere/active/dcos-ui-service/versions")
      The filesystem path where downloaded versions are stored.

      --master-count-file (default "/opt/mesosphere/etc/master_count")
      The filesystem path to the file determining the master count.

      --log-level (default "info")
      The output logging level.

      --http-client-timeout (default 5s)
      The default http client timeout for requests.

      --zk-addr (default "127.0.0.1:2181")
      The Zookeeper address this client will connect to.

      --zk-base-path (default "/dcos/ui-update")
      The path of the root zookeeper znode.

      --zk-auth-info
      Authentication details for zookeeper.

      --zk-znode-owner
      The ZK owner of the base path.

      --zk-session-timeout (default 5s)
      ZK session timeout duration.

      --zk-connect-timeout (default 5s)
      Timeout duration to establish initial zookeeper connection.

      --zk-poll-int duration (default 30s)
      Interval duration to check zookeeper node for version updates.

      --init-ui-dist-symlink
      Initialize the UI dist symlink if missing (Use for local development)
```

In addition, the following environment variables can also be used to configure similarly-named options:

```
DCOS_UI_UPDATE_LISTEN_ADDR
DCOS_UI_UPDATE_DEFAULT_UI_PATH
DCOS_UI_UPDATE_VERSIONS_ROOT
DCOS_UI_UPDATE_DIST_LINK
DCOS_UI_UPDATE_STAGE_LINK
DCOS_UI_UPDATE_ZK_AUTH_INFO
DCOS_UI_UPDATE_ZK_ZKNODE_OWNER
```

## Development

### With docker

The easiest way to develop on this project is to use the make file (which relies on docker).
For example `make test` will run linting and tests and it will run all these commands inside a docker container.

### Inside docker

You can run the service inside docker by exporting `$CLUSTER_URL`, `$AUTH_TOKEN` and running `make start`

```bash
$ export CLUSTER_URL=<path_to_a_cluster>
$ export AUTH_TOKEN=<token>
$ docker-compose up
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
