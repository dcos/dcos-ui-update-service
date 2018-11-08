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

	t.Run("default CACertFile is correct", func(t *testing.T) {
		cfg := Parse([]string{})

		tests.H(t).StringEql(cfg.CACertFile, "")
	})

	t.Run("sets CACertFile from cli arg", func(t *testing.T) {
		cfg := Parse([]string{"-" + optCaCert, "./testdata/ca-cert"})

		tests.H(t).StringEql(cfg.CACertFile, "./testdata/ca-cert")
	})

	t.Run("default IAMConfig is correct", func(t *testing.T) {
		cfg := Parse([]string{})

		tests.H(t).StringEql(cfg.IAMConfig, "")
	})

	t.Run("sets IAMConfig from cli arg", func(t *testing.T) {
		cfg := Parse([]string{"-" + optIAMConfig, "./testdata/iam-config"})

		tests.H(t).StringEql(cfg.IAMConfig, "./testdata/iam-config")
	})
}
