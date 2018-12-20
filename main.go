package main

import (
	"net"
	"os"

	"github.com/coreos/go-systemd/activation"
	"github.com/dcos/dcos-ui-update-service/config"
	"github.com/dcos/dcos-ui-update-service/uiService"
	"github.com/sirupsen/logrus"
)

// TODO: think about client timeouts https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/
func main() {
	cliArgs := os.Args[1:]
	config := config.Parse(cliArgs)

	initLogging(config)

	service, err := uiService.SetupService(config)
	if err != nil {
		logrus.WithFields(logrus.Fields{"err": err.Error()}).Fatal("Failed to initiate ui service")
	}

	listener := listener(service.Config)

	if err := service.Run(listener); err != nil {
		logrus.WithFields(logrus.Fields{"err": err.Error()}).Fatal("Application error")
	}
}

func listener(config *config.Config) net.Listener {
	// Use systemd socket activation.
	l, err := activation.Listeners()
	if err != nil {
		logrus.WithFields(logrus.Fields{"err": err.Error()}).Fatal("Failed to activate listeners from systemd")
	}

	var listener net.Listener
	switch numListeners := len(l); numListeners {
	case 0:
		logrus.Info("Did not receive any listeners from systemd, will start with configured listener instead.")
		listener, err = net.Listen(config.ListenNetProtocol, config.ListenNetAddress)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"connections": config.ListenNetProtocol,
				"address":     config.ListenNetAddress,
				"err":         err.Error(),
			}).Fatal("Cannot listen for connections")
		}
		logrus.WithFields(logrus.Fields{"net": config.ListenNetProtocol, "Addr": config.ListenNetAddress}).Info("Listening")
	case 1:
		listener = l[0]
		logrus.WithFields(logrus.Fields{"socket": listener.Addr()}).Info("Listening on systemd")
	default:
		logrus.Fatal("Found multiple systemd sockets.")
	}
	return listener
}
