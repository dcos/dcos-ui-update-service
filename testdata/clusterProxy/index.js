const fs = require("fs");
const httpProxy = require('http-proxy');

const port = 6000;
let clusterUrl = process.env.CLUSTER_URL || "";
const authToken = process.env.AUTH_TOKEN || "";

if (clusterUrl === "") {
    console.error("$CLUSTER_URL variable not set, must have cluster to proxy to");
    process.exit(1);
}

if (clusterUrl.startsWith("https://")) {
    clusterUrl = clusterUrl.replace("https://", "http://");
}

if (authToken === "") {
    console.error("$AUTH_TOKEN variable not set, must have auth token to make calls to cluster");
    process.exit(1);
}

const proxyOptions = {
    target: clusterUrl,
    changeOrigin: false,
    secure: false,
    headers: { authorization: `token=${authToken}` },
};

console.log(`Starting proxy server to ${clusterUrl} on port:${port}`);
const proxy = httpProxy.createProxyServer(proxyOptions);
proxy.listen(port);

proxy.on('proxyRes', function (proxyRes, req, res) {
    console.log(`Handling: ${req.url}`);
});