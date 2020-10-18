package shell

import (
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField("module", "shell")

type Mapping struct {
	Cc      int    `yaml:"cc"`
	Command string `yaml:"command"`
}

func (m *Mapping) Cleanup() {
	log.Trace("Enter Cleanup")
	defer log.Trace("Exit Cleanup")
}

func (m *Mapping) Initialize() {
	log.Trace("Enter Initialize")
	defer log.Trace("Exit Initialize")
}

func (m *Mapping) HandleMessage(c int, v int) {
	if m.Cc != c {
		return
	}

	log.Infof("Not implemented yet, but this would execute %s", m.Command)
}
