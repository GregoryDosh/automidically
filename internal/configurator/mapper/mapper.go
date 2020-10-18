package mapper

import (
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField("module", "mapper")

type Instance struct {
	Volume map[int][]Volume
}

func (i *Instance) UnmarshalYAML(unmarshal func(interface{}) error) error {
	return nil
}
