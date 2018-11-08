package main

import (
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
)

// DCOS handles access to common setup question
type DCOS struct {
	MasterCountLocation string
}

type MasterCounter interface {
	IsMultiMaster() (bool, error)
}

// IsMultiMaster returns true if there is more than one master node
func (d DCOS) IsMultiMaster() (bool, error) {
	file, err := ioutil.ReadFile(d.MasterCountLocation)

	if err != nil {
		return false, fmt.Errorf("Could not find %q on file system", d.MasterCountLocation)
	}

	content := string(file)
	content = strings.TrimSuffix(content, "\n")
	number, err := strconv.ParseInt(content, 10, 0)

	if err != nil {
		return false, fmt.Errorf("The file could not be parsed: %q", d.MasterCountLocation)
	}

	return number > 1, nil
}
