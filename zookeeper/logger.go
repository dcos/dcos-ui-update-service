package zookeeper

import (
	"github.com/samuel/go-zookeeper/zk"
)

// ZookeeperLogger custom logger for zk.Conn
type zookeeperLogger struct{}

// Printf handles logging zookeeper connection logs as Trace logs
func (zkLog *zookeeperLogger) Printf(format string, args ...interface{}) {
	log.Tracef(format, args...)
}

// ZookeeperClientLogger create a custom logger to be used with zk.Conn
func zookeeperClientLogger() zk.Logger {
	return &zookeeperLogger{}
}
