package midi

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"

	gomidi "gitlab.com/gomidi/midi"
	driver "gitlab.com/gomidi/rtmididrv"
)

var log = logrus.WithField("module", "midi")

type Device struct {
	driver       *driver.Driver
	device       gomidi.In
	deviceName   string
	subscribers  []chan [2]byte
	deviceIsOpen bool
	sync.Mutex
}

func New(deviceName string) (*Device, error) {
	log.Debug("gathering midi devices")

	if deviceName == "" {
		return nil, errors.New("missing midi deviceName")
	} else {
		log.Debugf("looking for %s", deviceName)
	}

	drv, err := driver.New()
	if err != nil {
		return nil, fmt.Errorf("unable to open midi driver: %w", err)
	}

	midiDevices, err := drv.Ins()
	if err != nil {
		return nil, fmt.Errorf("unable to open midi inputs: %w", err)
	}

	for _, d := range midiDevices {
		log.Debugf("found device %s", d.String())
		if strings.Contains(d.String(), deviceName) {
			log.Infof("using device %s", d.String())
			nd := &Device{
				driver:     drv,
				device:     d,
				deviceName: d.String(),
			}
			go nd.startCommunication()
			return nd, nil
		}
	}

	drv.Close()
	return nil, fmt.Errorf("unable to find midi input device: %s", deviceName)
}

func (d *Device) startCommunication() {
	err := d.device.Open()
	if err != nil {
		log.Error(err)
	}

	err = d.device.SetListener(d.dispatchMessage)
	if err != nil {
		log.Error(err)
	}
	d.deviceIsOpen = true
	log.Debugf("opened midi device %s", d.deviceName)
}

func (d *Device) Cleanup() {
	log.Debugf("stopping & cleaning up device %s", d.deviceName)
	d.Lock()
	defer d.Unlock()
	if d.device != nil {
		d.device.Close()
		d.device = nil
	}
	if d.driver != nil {
		d.driver.Close()
		d.driver = nil
	}
	for _, s := range d.subscribers {
		close(s)
	}
	d.subscribers = nil
}

func (d *Device) dispatchMessage(m []byte, t int64) {
	if len(m) == 3 {
		d.Lock()
		defer d.Unlock()
		for _, s := range d.subscribers {
			s <- [2]byte{m[1], m[2]}
		}
	}
}

func (d *Device) SubscribeToMessages() (chan [2]byte, error) {
	if d == nil {
		return nil, errors.New("MIDI Device Uninitialized")
	}
	c := make(chan [2]byte, 1)
	d.Lock()
	defer d.Unlock()
	d.subscribers = append(d.subscribers, c)
	return c, nil
}
