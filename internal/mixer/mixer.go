package mixer

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

var (
	log = logrus.WithField("module", "mixer")
)

type Mapping struct {
	Cc          int      `yaml:"cc"`
	HardwareMin int      `yaml:"hardwareMin"`
	HardwareMax int      `yaml:"hardwareMax"`
	VolumeMin   float32  `yaml:"volumeMin"`
	VolumeMax   float32  `yaml:"volumeMax"`
	Filename    []string `yaml:"-"`
	Special     []string `yaml:"-"`
	Device      []string `yaml:"-"`
}

func (m *Mapping) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// This is so we can set some default values if not specified in the config.
	type rawMapping Mapping
	raw := rawMapping{
		HardwareMin: 0,
		HardwareMax: 127,
		VolumeMin:   0,
		VolumeMax:   1,
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

func (m *Mapping) Validate() error {
	if m.HardwareMin > m.HardwareMax {
		return fmt.Errorf("hardware minimum %d should not be greater than maximum %d", m.HardwareMin, m.HardwareMax)
	}
	if m.VolumeMin < 0 || m.VolumeMin > 1 {
		return fmt.Errorf("volume minimum %f should be in range [0,1]", m.VolumeMin)
	}
	if m.VolumeMax < 0 || m.VolumeMax > 1 {
		return fmt.Errorf("volume maximum %f should be in range [0,1]", m.VolumeMax)
	}
	return nil
}
