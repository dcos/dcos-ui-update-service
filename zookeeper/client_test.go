package zookeeper

import (
	"testing"
	"time"

	"github.com/dcos/dcos-ui-update-service/config"
	"github.com/dcos/dcos-ui-update-service/tests"
)

func TestZookeeper(t *testing.T) {
	t.Run("client can connect to zk", func(t *testing.T) {
		zkControl, err := tests.StartZookeeper()

		tests.H(t).IsNil(err)
		defer zkControl.Teardown()

		if zkControl.Addr() == "" {
			t.Errorf("expected zookeeper address to be available")
		}
		client, err := Connect(&config.Config{
			ZKBasePath:          "/dcos/ui-update",
			ZKZnodeOwner:        "",
			ZKAuthInfo:          "",
			ZKAddress:           zkControl.Addr(),
			ZKSessionTimeout:    10 * time.Second,
			ZKConnectionTimeout: 10 * time.Second,
		})
		tests.H(t).IsNil(err)
		client.Close()
	})
}
