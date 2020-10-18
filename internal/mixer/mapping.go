package mixer

import (
	"math"
	"strings"
)

type Mapping struct {
	Cc       int      `yaml:"cc"`
	Reverse  bool     `yaml:"reverse,omitempty"`
	Min      float64  `yaml:"min,omitempty"`
	Max      float64  `yaml:"max,omitempty"`
	Filename []string `yaml:"filename,omitempty"`
	Special  []string `yaml:"special,omitempty"`
	Device   []string `yaml:"device,omitempty"`
}

func (m *Mapping) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// This is so we can set some default values if not specified in the config.
	type rawMapping Mapping
	raw := rawMapping{
		Reverse: false,
		Min:     0,
		Max:     127,
	}
	if err := unmarshal(&raw); err != nil {
		return err
	}

	*m = Mapping(raw)
	return nil
}

func (m *Mapping) Cleanup() {
	mpLog.Trace("Enter Cleanup")
	defer mpLog.Trace("Exit Cleanup")
}

func (m *Mapping) Initialize() {
	mpLog.Trace("Enter Initialize")
	defer mpLog.Trace("Exit Initialize")
}

func (m *Mapping) HandleMessage(c int, v int) {
	if m.Cc != c {
		return
	}

	valueRange := m.Max - m.Min
	clampedValue := math.Max(m.Min, math.Min(m.Max, float64(v)))
	newValue := float32(clampedValue / valueRange)
	if m.Reverse {
		newValue = 1 - newValue
	}

	// special
	for _, s := range m.Special {
		// output
		if strings.EqualFold(s, "output") {
			das.outputDevice.SetVolumeLevel(newValue)
		}
		// input
		if strings.EqualFold(s, "input") {
			das.inputDevice.SetVolumeLevel(newValue)
		}
		// system
		if strings.EqualFold(s, "system") {
			mpLog.Infof("System Volume Not Setup Yet %f", newValue)
		}
	}

	// filename
	for _, f := range m.Filename {
		mpLog.Infof("Filename %s Logging Not Setup Yet %f", f, newValue)
	}

	// device
	for _, f := range m.Device {
		mpLog.Infof("Device %s Logging Not Setup Yet %f", f, newValue)
	}
}