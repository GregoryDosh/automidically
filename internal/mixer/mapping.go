package mixer

import (
	"math"
	"strings"
)

type Mapping struct {
	Cc       int     `yaml:"cc"`
	Reverse  bool    `yaml:"reverse,omitempty"`
	Min      float64 `yaml:"min,omitempty"`
	Max      float64 `yaml:"max,omitempty"`
	Filename string  `yaml:"filename,omitempty"`
	Special  string  `yaml:"special,omitempty"`
	Device   string  `yaml:"device,omitempty"`
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

	clampedValue := math.Max(m.Min, math.Min(m.Max, float64(v)))
	newValue := float32(clampedValue / m.Max)
	if m.Reverse {
		newValue = 1 - newValue
	}

	// special
	if m.Special != "" {
		// refresh_devices
		if strings.EqualFold(m.Special, "refresh_devices") {
			das.refreshDevices <- true
		}
		// refresh_sessions
		if strings.EqualFold(m.Special, "refresh_sessions") {
			das.refreshSessions <- true
		}
		// output
		if strings.EqualFold(m.Special, "output") {
			if das.outputDevice != nil {
				das.outputDevice.SetVolumeLevel(newValue)
			}
		}
		// input
		if strings.EqualFold(m.Special, "input") {
			if das.inputDevice != nil {
				das.inputDevice.SetVolumeLevel(newValue)
			}
		}
		// system
		if strings.EqualFold(m.Special, "system") {
			if das.systemSession != nil {
				das.systemSession.SetVolume(newValue)
			}
		}
	}

	// filename
	if m.Filename != "" {
		changeSessionVolume(m.Filename, newValue)
	}

	// device
	if m.Device != "" {
		mpLog.Infof("Device %s Logging Not Setup Yet %f", m.Device, newValue)
	}
}
