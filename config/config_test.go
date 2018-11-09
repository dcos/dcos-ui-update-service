package config

import (
	"testing"
	"time"

	"github.com/dcos/dcos-ui-update-service/tests"
)

func TestConfig(t *testing.T) {
	t.Run("default ListenNetProtocol is correct", func(t *testing.T) {
		cfg := Parse([]string{})

		tests.H(t).StringEql(cfg.ListenNetProtocol, defaultListenNet)
	})

	t.Run("sets ListenNetProtocol from cli arg", func(t *testing.T) {
		cfg := Parse([]string{"-" + optListenNet, "tcp"})

		tests.H(t).StringEql(cfg.ListenNetProtocol, "tcp")
	})

	t.Run("default ListenNetAddress is correct", func(t *testing.T) {
		cfg := Parse([]string{})

		tests.H(t).StringEql(cfg.ListenNetAddress, defaultListenAddr)
	})

	t.Run("sets ListenNetAddress from cli arg", func(t *testing.T) {
		cfg := Parse([]string{"-" + optListenAddress, "0.0.0.0:5000"})

		tests.H(t).StringEql(cfg.ListenNetAddress, "0.0.0.0:5000")
	})

	t.Run("default StaticAssetPrefix is correct", func(t *testing.T) {
		cfg := Parse([]string{})

		tests.H(t).StringEql(cfg.StaticAssetPrefix, defaultAssetPrefix)
	})

	t.Run("sets StaticAssetPrefix from cli arg", func(t *testing.T) {
		cfg := Parse([]string{"-" + optAssetPrefix, "/"})

		tests.H(t).StringEql(cfg.StaticAssetPrefix, "/")
	})

	t.Run("default UniverseURL is correct", func(t *testing.T) {
		cfg := Parse([]string{})

		tests.H(t).StringEql(cfg.UniverseURL, defaultUniverseURL)
	})

	t.Run("sets UniverseURL from cli arg", func(t *testing.T) {
		cfg := Parse([]string{"-" + optUniverseURL, "https://unit-test.mesosphere.com"})

		tests.H(t).StringEql(cfg.UniverseURL, "https://unit-test.mesosphere.com")
	})

	t.Run("default DefaultDocRoot is correct", func(t *testing.T) {
		cfg := Parse([]string{})

		tests.H(t).StringEql(cfg.DefaultDocRoot, defaultDefaultDocRoot)
	})

	t.Run("sets DefaultDocRoot from cli arg", func(t *testing.T) {
		cfg := Parse([]string{"-" + optDefaultDocRoot, "./testdata/docroot"})

		tests.H(t).StringEql(cfg.DefaultDocRoot, "./testdata/docroot")
	})

	t.Run("default VersionsRoot is correct", func(t *testing.T) {
		cfg := Parse([]string{})

		tests.H(t).StringEql(cfg.VersionsRoot, defaultVersionsRoot)
	})

	t.Run("sets VersionsRoot from cli arg", func(t *testing.T) {
		cfg := Parse([]string{"-" + optVersionsRoot, "./testdata/versions"})

		tests.H(t).StringEql(cfg.VersionsRoot, "./testdata/versions")
	})

	t.Run("default MasterCountFile is correct", func(t *testing.T) {
		cfg := Parse([]string{})

		tests.H(t).StringEql(cfg.MasterCountFile, defaultMasterCountFile)
	})

	t.Run("sets MasterCountFile from cli arg", func(t *testing.T) {
		cfg := Parse([]string{"-" + optMasterCountFile, "./testdata"})

		tests.H(t).StringEql(cfg.MasterCountFile, "./testdata")
	})

	t.Run("default HTTPClientTimeout is correct", func(t *testing.T) {
		cfg := Parse([]string{})

		tests.H(t).Int64Eql(cfg.HTTPClientTimeout.Nanoseconds(), defaultHTTPClientTimeout.Nanoseconds())
	})

	t.Run("sets HTTPClientTimeout from cli arg", func(t *testing.T) {
		cfg := Parse([]string{"-" + optHTTPClientTimeout, "10s"})

		tests.H(t).Int64Eql(cfg.HTTPClientTimeout.Nanoseconds(), (10 * time.Second).Nanoseconds())
	})

	t.Run("default LogLevel is correct", func(t *testing.T) {
		cfg := Parse([]string{})

		tests.H(t).StringEql(cfg.LogLevel, "info")
	})

	t.Run("sets LogLevel from cli arg", func(t *testing.T) {
		cfg := Parse([]string{"-" + optLogLevel, "error"})

		tests.H(t).StringEql(cfg.LogLevel, "error")
	})

	t.Run("default ZKAddress is correct", func(t *testing.T) {
		cfg := Parse([]string{})

		tests.H(t).StringEql(cfg.ZKAddress, defaultZKAddress)
	})

	t.Run("sets ZKAddress from cli arg", func(t *testing.T) {
		cfg := Parse([]string{"-" + optZKAddress, "0.0.0.0:2181"})

		tests.H(t).StringEql(cfg.ZKAddress, "0.0.0.0:2181")
	})

	t.Run("default ZKBasePath is correct", func(t *testing.T) {
		cfg := Parse([]string{})

		tests.H(t).StringEql(cfg.ZKBasePath, defaultZKBasePath)
	})

	t.Run("sets ZKBasePath from cli arg", func(t *testing.T) {
		cfg := Parse([]string{"-" + optZKBasePath, "0.0.0.0:2181"})

		tests.H(t).StringEql(cfg.ZKBasePath, "0.0.0.0:2181")
	})

	t.Run("default ZKAuthInfo is correct", func(t *testing.T) {
		cfg := Parse([]string{})

		tests.H(t).StringEql(cfg.ZKAuthInfo, defaultZKAuthInfo)
	})

	t.Run("sets ZKAuthInfo from cli arg", func(t *testing.T) {
		cfg := Parse([]string{"-" + optZKAuthInfo, "test-thing"})

		tests.H(t).StringEql(cfg.ZKAuthInfo, "test-thing")
	})

	t.Run("default ZKZnodeOwner is correct", func(t *testing.T) {
		cfg := Parse([]string{})

		tests.H(t).StringEql(cfg.ZKZnodeOwner, defaultZKZnodeOwner)
	})

	t.Run("sets ZKZnodeOwner from cli arg", func(t *testing.T) {
		cfg := Parse([]string{"-" + optZKZnodeOwner, "testOwner"})

		tests.H(t).StringEql(cfg.ZKZnodeOwner, "testOwner")
	})

	t.Run("default ZKSessionTimeout is correct", func(t *testing.T) {
		cfg := Parse([]string{})

		tests.H(t).Int64Eql(cfg.ZKSessionTimeout.Nanoseconds(), defaultZKSessionTimeout.Nanoseconds())
	})

	t.Run("sets ZKSessionTimeout from cli arg", func(t *testing.T) {
		cfg := Parse([]string{"-" + optZKSessionTimeout, "20s"})

		tests.H(t).Int64Eql(cfg.ZKSessionTimeout.Nanoseconds(), (20 * time.Second).Nanoseconds())
	})

	t.Run("default ZKConnectionTimeout is correct", func(t *testing.T) {
		cfg := Parse([]string{})

		tests.H(t).Int64Eql(cfg.ZKConnectionTimeout.Nanoseconds(), defaultZKConnectTimeout.Nanoseconds())
	})

	t.Run("sets ZKConnectionTimeout from cli arg", func(t *testing.T) {
		cfg := Parse([]string{"-" + optZKConnectTimeout, "20s"})

		tests.H(t).Int64Eql(cfg.ZKConnectionTimeout.Nanoseconds(), (20 * time.Second).Nanoseconds())
	})
}
