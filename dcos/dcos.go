package dcos

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/dcos/dcos-go/dcos"
	"github.com/dcos/dcos-go/dcos/nodeutil"
	"github.com/dcos/dcos-ui-update-service/config"
)

// DCOS handles access to common setup question
type dcosHelper struct {
	MasterCountLocation string
	nodeInfo            nodeutil.NodeInfo
}

type DCOS interface {
	IsMultiMaster() (bool, error)
	MasterCount() (uint64, error)
	MesosID() (string, error)
	DetectIP() (net.IP, error)
	IsLeader() (bool, error)
}

func NewDCOS(cfg *config.Config) DCOS {
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
	d, err := nodeutil.NewNodeInfo(client, dcos.RoleMaster, dcosOptions(cfg)...)
	if err != nil {
		panic(err)
	}

	return &dcosHelper{
		MasterCountLocation: cfg.MasterCountFile(),
		nodeInfo:            d,
	}
}

func dcosOptions(cfg *config.Config) []nodeutil.Option {
	var options []nodeutil.Option

	mesosStateURL := cfg.MesosStateURL()
	if mesosStateURL != "" {
		options = append(options, nodeutil.OptionMesosStateURL(mesosStateURL))
	}

	return options
}

// IsMultiMaster returns true if there is more than one master node
func (d dcosHelper) IsMultiMaster() (bool, error) {
	number, err := d.MasterCount()
	if err != nil {
		return false, err
	}

	return number > 1, nil
}

// MasterCount returns the expected number of masters
func (d dcosHelper) MasterCount() (uint64, error) {
	file, err := ioutil.ReadFile(d.MasterCountLocation)

	if err != nil {
		return 0, fmt.Errorf("Could not find %q on file system", d.MasterCountLocation)
	}

	content := string(file)
	content = strings.TrimSuffix(content, "\n")
	number, err := strconv.ParseUint(content, 10, 0)

	if err != nil {
		return 0, fmt.Errorf("The file could not be parsed: %q", d.MasterCountLocation)
	}
	return number, nil
}

func (d dcosHelper) MesosID() (string, error) {
	return d.nodeInfo.MesosID(nil)
}

func (d dcosHelper) DetectIP() (net.IP, error) {
	return d.nodeInfo.DetectIP()
}

func (d dcosHelper) IsLeader() (bool, error) {
	return d.nodeInfo.IsLeader()
}
