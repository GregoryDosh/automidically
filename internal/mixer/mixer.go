package mixer

import (
	"github.com/sirupsen/logrus"
)

var (
	log = logrus.WithField("module", "mixer")
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
