package mixer

import (
	"math"
	"strings"
)

type Mapping struct {
	Cc       int      `yaml:"cc"`
	Reverse  bool     `yaml:"reverse"`
	Min      float64  `yaml:"min"`
	Max      float64  `yaml:"max"`
	Filename []string `yaml:"-"`
	Special  []string `yaml:"-"`
	Device   []string `yaml:"-"`
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

	// This is kludgy, but with it we can infer the params as strings or slices.
	{
		rString := struct {
			Filename string
			Special  string
			Device   string
		}{}
		_ = unmarshal(&rString)
		if rString.Filename != "" {
			raw.Filename = []string{rString.Filename}
		}
		if rString.Special != "" {
			raw.Special = []string{rString.Special}
		}
		if rString.Device != "" {
			raw.Device = []string{rString.Device}
		}
		rSlice := struct {
			Filename []string
			Special  []string
			Device   []string
		}{}
		_ = unmarshal(&rSlice)
		if len(rSlice.Filename) > 0 {
			raw.Filename = rSlice.Filename
		}
		if len(rSlice.Special) > 0 {
			raw.Special = rSlice.Special
		}
		if len(rSlice.Device) > 0 {
			raw.Device = rSlice.Device
		}
	}

	*m = Mapping(raw)
	return nil
}

func (m *Mapping) Cleanup() {
	mpLog.Trace("Enter Cleanup")
	defer mpLog.Trace("Exit Cleanup")
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
	for _, s := range m.Special {
		// refresh_devices
		if strings.EqualFold(s, "refresh_devices") {
			das.refreshDevices <- true
		}
		// refresh_sessions
		if strings.EqualFold(s, "refresh_sessions") {
			das.refreshSessions <- true
		}
		// output
		if strings.EqualFold(s, "output") {
			if das.outputDevice != nil {
				das.outputDevice.SetVolumeLevel(newValue)
			}
		}
		// input
		if strings.EqualFold(s, "input") {
			if das.inputDevice != nil {
				das.inputDevice.SetVolumeLevel(newValue)
			}
		}
		// system
		if strings.EqualFold(s, "system") {
			if das.systemSession != nil {
				das.systemSession.SetVolume(newValue)
			}
		}
	}

	// filename
	for _, f := range m.Filename {
		changeSessionVolume(f, newValue)
	}

	// device
	for _, d := range m.Device {
		mpLog.Infof("Device Control Not Implemented: %s", d)
	}
}
