package config

import (
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/dcos/dcos-ui-update-service/tests"
)

func TestDefaultConfig(t *testing.T) {
	t.Run("defaults are intended values", func(t *testing.T) {
		defaults := NewDefaultConfig()

		helper := tests.H(t)
		helper.StringEql(defaults.ConfigFilePath(), defaultConfig)
		helper.StringEql(defaults.DefaultDocRoot(), defaultDefaultDocRoot)
		helper.Int64Eql(defaults.HTTPClientTimeout().Nanoseconds(), defaultHTTPClientTimeout.Nanoseconds())
		helper.StringEql(defaults.ListenNetProtocol(), defaultListenNet)
		helper.StringEql(defaults.ListenNetAddress(), defaultListenAddr)
		helper.StringEql(defaults.UniverseURL(), defaultUniverseURL)
		helper.StringEql(defaults.UIDistSymlink(), defaultUIDistSymlink)
		helper.StringEql(defaults.UIDistStageSymlink(), defaultUIDistStageSymlink)
		helper.StringEql(defaults.VersionsRoot(), defaultVersionsRoot)
		helper.StringEql(defaults.MasterCountFile(), defaultMasterCountFile)
		helper.StringEql(defaults.LogLevel(), defaultLogLevel)
		helper.StringEql(defaults.ZKAddress(), defaultZKAddress)
		helper.StringEql(defaults.ZKBasePath(), defaultZKBasePath)
		helper.StringEql(defaults.ZKAuthInfo(), defaultZKAuthInfo)
		helper.StringEql(defaults.ZKZnodeOwner(), defaultZKZnodeOwner)
		helper.Int64Eql(defaults.ZKSessionTimeout().Nanoseconds(), defaultZKSessionTimeout.Nanoseconds())
		helper.Int64Eql(defaults.ZKConnectionTimeout().Nanoseconds(), defaultZKConnectTimeout.Nanoseconds())
		helper.Int64Eql(defaults.ZKPollingInterval().Nanoseconds(), defaultZKPollingInterval.Nanoseconds())
		helper.BoolEql(defaults.InitUIDistSymlink(), defaultInitUIDistSymlink)
	})
}

func TestEnvironmentConfig(t *testing.T) {
	t.Run("sets ListenNetAddress from ENV", func(t *testing.T) {
		if os.Getenv("BE_CONFIG_ENV_TEST") == "1" {
			cfg, _ := Parse(nil)
			tests.H(t).StringEql(cfg.ListenNetAddress(), "123.123.123.123:123")
			os.Exit(0)
			return
		}

		cmd := exec.Command(os.Args[0], "-test.run=TestEnvironmentConfig")
		cmd.Env = append(os.Environ(), "DCOS_UI_UPDATE_LISTEN_ADDR=123.123.123.123:123", "BE_CONFIG_ENV_TEST=1")
		err := cmd.Run()
		tests.H(t).IsNil(err)
	})
}

func TestConfigFile(t *testing.T) {
	t.Run("sets ListenNetAddress from config file", func(t *testing.T) {
		cfg, _ := Parse([]string{"--config", "../fixtures/config.json"})
		tests.H(t).StringEql(cfg.ListenNetAddress(), "255.255.255.1:3000")
		tests.H(t).StringEql(cfg.ListenNetProtocol(), "tcp")
	})

	t.Run("CLI args should override config file", func(t *testing.T) {
		cfg, _ := Parse([]string{"--config", "../fixtures/config.json", "--listen-addr", "1.1.1.255:3333"})
		tests.H(t).StringEql(cfg.ListenNetAddress(), "1.1.1.255:3333")
	})
}

func TestConfig(t *testing.T) {
	t.Run("can parse ListenNetAddress", func(t *testing.T) {
		cfg, err := Parse([]string{"--" + optListenAddress, "0.0.0.0:5000"})

		helper := tests.H(t)
		helper.IsNil(err)
		helper.StringEql(cfg.ListenNetAddress(), "0.0.0.0:5000")
	})

	t.Run("sets ZKConnectionTimeout from cli arg", func(t *testing.T) {
		cfg, err := Parse([]string{"--" + optZKConnectTimeout, "20s"})

		helper := tests.H(t)
		helper.IsNil(err)
		helper.Int64Eql(cfg.ZKConnectionTimeout().Nanoseconds(), (20 * time.Second).Nanoseconds())
	})

	t.Run("sets ZKPollingInterval from cli arg", func(t *testing.T) {
		cfg, err := Parse([]string{"--" + optZKPollingInterval, "20s"})

		helper := tests.H(t)
		helper.IsNil(err)
		helper.Int64Eql(cfg.ZKPollingInterval().Nanoseconds(), (20 * time.Second).Nanoseconds())
	})

	t.Run("sets ListenNetProtocol from cli arg", func(t *testing.T) {
		cfg, err := Parse([]string{"--" + optListenNet, "tcp"})

		helper := tests.H(t)
		helper.IsNil(err)
		helper.StringEql(cfg.ListenNetProtocol(), "tcp")
	})

	t.Run("sets ListenNetAddress from cli arg", func(t *testing.T) {
		cfg, err := Parse([]string{"--" + optListenAddress, "0.0.0.0:5000"})

		helper := tests.H(t)
		helper.IsNil(err)
		helper.StringEql(cfg.ListenNetAddress(), "0.0.0.0:5000")
	})

	t.Run("sets UniverseURL from cli arg", func(t *testing.T) {
		cfg, err := Parse([]string{"--" + optUniverseURL, "https://unit-test.mesosphere.com"})

		helper := tests.H(t)
		helper.IsNil(err)
		helper.StringEql(cfg.UniverseURL(), "https://unit-test.mesosphere.com")
	})

	t.Run("sets DefaultDocRoot from cli arg", func(t *testing.T) {
		cfg, err := Parse([]string{"--" + optDefaultDocRoot, "./testdata/docroot"})

		helper := tests.H(t)
		helper.IsNil(err)
		helper.StringEql(cfg.DefaultDocRoot(), "./testdata/docroot")
	})

	t.Run("sets UIDistSymlink from cli arg", func(t *testing.T) {
		cfg, err := Parse([]string{"--" + optUIDistSymlink, "./testdata/ui-dist"})

		helper := tests.H(t)
		helper.IsNil(err)
		helper.StringEql(cfg.UIDistSymlink(), "./testdata/ui-dist")
	})

	t.Run("sets UIDistSymlink from cli arg", func(t *testing.T) {
		cfg, err := Parse([]string{"--" + optUIDistStageSymlink, "./testdata/new-ui-dist"})

		helper := tests.H(t)
		helper.IsNil(err)
		helper.StringEql(cfg.UIDistStageSymlink(), "./testdata/new-ui-dist")
	})

	t.Run("sets VersionsRoot from cli arg", func(t *testing.T) {
		cfg, err := Parse([]string{"--" + optVersionsRoot, "./testdata/versions"})

		helper := tests.H(t)
		helper.IsNil(err)
		helper.StringEql(cfg.VersionsRoot(), "./testdata/versions")
	})

	t.Run("sets MasterCountFile from cli arg", func(t *testing.T) {
		cfg, err := Parse([]string{"--" + optMasterCountFile, "./testdata"})

		helper := tests.H(t)
		helper.IsNil(err)
		helper.StringEql(cfg.MasterCountFile(), "./testdata")
	})

	t.Run("sets HTTPClientTimeout from cli arg", func(t *testing.T) {
		cfg, err := Parse([]string{"--" + optHTTPClientTimeout, "10s"})

		helper := tests.H(t)
		helper.IsNil(err)
		helper.Int64Eql(cfg.HTTPClientTimeout().Nanoseconds(), (10 * time.Second).Nanoseconds())
	})

	t.Run("sets LogLevel from cli arg", func(t *testing.T) {
		cfg, err := Parse([]string{"--" + optLogLevel, "error"})

		helper := tests.H(t)
		helper.IsNil(err)
		helper.StringEql(cfg.LogLevel(), "error")
	})

	t.Run("sets ZKAddress from cli arg", func(t *testing.T) {
		cfg, err := Parse([]string{"--" + optZKAddress, "0.0.0.0:2181"})

		helper := tests.H(t)
		helper.IsNil(err)
		helper.StringEql(cfg.ZKAddress(), "0.0.0.0:2181")
	})

	t.Run("sets ZKBasePath from cli arg", func(t *testing.T) {
		cfg, err := Parse([]string{"--" + optZKBasePath, "0.0.0.0:2181"})

		helper := tests.H(t)
		helper.IsNil(err)
		helper.StringEql(cfg.ZKBasePath(), "0.0.0.0:2181")
	})

	t.Run("sets ZKAuthInfo from cli arg", func(t *testing.T) {
		cfg, err := Parse([]string{"--" + optZKAuthInfo, "test-thing"})

		helper := tests.H(t)
		helper.IsNil(err)
		helper.StringEql(cfg.ZKAuthInfo(), "test-thing")
	})

	t.Run("sets ZKZnodeOwner from cli arg", func(t *testing.T) {
		cfg, err := Parse([]string{"--" + optZKZnodeOwner, "testOwner"})

		helper := tests.H(t)
		helper.IsNil(err)
		helper.StringEql(cfg.ZKZnodeOwner(), "testOwner")
	})

	t.Run("sets ZKSessionTimeout from cli arg", func(t *testing.T) {
		cfg, err := Parse([]string{"--" + optZKSessionTimeout, "20s"})

		helper := tests.H(t)
		helper.IsNil(err)
		helper.Int64Eql(cfg.ZKSessionTimeout().Nanoseconds(), (20 * time.Second).Nanoseconds())
	})

	t.Run("sets ZKConnectionTimeout from cli arg", func(t *testing.T) {
		cfg, err := Parse([]string{"--" + optZKConnectTimeout, "20s"})

		helper := tests.H(t)
		helper.IsNil(err)
		helper.Int64Eql(cfg.ZKConnectionTimeout().Nanoseconds(), (20 * time.Second).Nanoseconds())
	})

	t.Run("sets ZKPollingInterval from cli arg", func(t *testing.T) {
		cfg, err := Parse([]string{"--" + optZKPollingInterval, "20s"})

		helper := tests.H(t)
		helper.IsNil(err)
		helper.Int64Eql(cfg.ZKPollingInterval().Nanoseconds(), (20 * time.Second).Nanoseconds())
	})

	t.Run("sets InitUIDistSymlink from cli arg", func(t *testing.T) {
		cfg, err := Parse([]string{"--" + optInitUIDistSymlink, "true"})

		helper := tests.H(t)
		helper.IsNil(err)
		helper.BoolEql(cfg.InitUIDistSymlink(), true)
	})

	t.Run("returns ErrPotentiallyDangerousVersionsRoot when versions-root is empty", func(t *testing.T) {
		_, err := Parse([]string{"--" + optVersionsRoot, ""})
		tests.H(t).NotNil(err)
		tests.H(t).StringContains(err.Error(), "potentially dangerous versions-root")
	})
}
