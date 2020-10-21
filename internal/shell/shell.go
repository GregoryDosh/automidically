package shell

import (
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField("module", "shell")

type Mapping struct {
	Cc      int      `yaml:"cc"`
	Command []string `yaml:"-"`
}

func (m *Mapping) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// This is so we can set some default values if not specified in the config.
	type rawMapping Mapping
	var raw rawMapping
	if err := unmarshal(&raw); err != nil {
		return err
	}
	*m = Mapping(raw)

	// Letting command take on a string or string slice
	// requires a little bit of massaging to get into the struct.
	cString := struct{ Command string }{}
	if err := unmarshal(&cString); err == nil {
		m.Command = []string{cString.Command}
	}

	cSlice := struct{ Command []string }{}
	if err := unmarshal(&cSlice); err == nil {
		m.Command = cSlice.Command
	}

	return nil
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

	log.Infof("Shell Actions Not Implemented: %s", m.Command)
}
