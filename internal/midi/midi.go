package midi

import (
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
	gomidi "gitlab.com/gomidi/midi"
	driver "gitlab.com/gomidi/rtmididrv"
)

var log = logrus.WithField("module", "midi")

type Device struct {
	DeviceName string

	messageCallback func(int, int)
	messageChan     chan [2]int
	driver          *driver.Driver
	device          gomidi.In
	sync.Mutex
}

func (d *Device) Cleanup() error {
	d.Lock()
	defer d.Unlock()
	if d.messageChan != nil {
		close(d.messageChan)
		d.messageChan = nil
	}
	if d.device != nil {
		d.device.Close()
		d.device = nil
	}
	if d.driver != nil {
		d.driver.Close()
		d.driver = nil
	}
	if d.messageCallback != nil {
		d.messageCallback = nil
	}
	return nil
}

func (d *Device) SetMessageCallback(cb func(int, int)) {
	d.Lock()
	defer d.Unlock()
	d.messageCallback = cb
}

func (d *Device) handleMIDIMessageLoop() {
	log.Trace("Enter handleMIDIMessageLoop")
	defer log.Trace("Exit handleMIDIMessageLoop")

	if err := d.device.Open(); err != nil {
		log.Error(err)
	}

	if err := d.device.SetListener(func(data []byte, deltaMicroseconds int64) {
		if len(data) == 3 {
			d.messageChan <- [2]int{int(data[1]), int(data[2])}
		}
	}); err != nil {
		log.Error(err)
	}

	for msg := range d.messageChan {
		d.Lock()
		if d.messageCallback != nil {
			d.messageCallback(msg[0], msg[1])
		}
		d.Unlock()
	}
}

func New(searchName string) *Device {
	if searchName == "" {
		log.Error("missing MIDI device name")
		return nil
	}

	drv, err := driver.New()
	if err != nil {
		log.Errorf("unable to open midi driver: %s", err)
		return nil
	}

	midiInputs, err := drv.Ins()
	if err != nil {
		log.Errorf("unable to open midi inputs: %s", err)
		return nil
	}

	for _, in := range midiInputs {
		log.Debugf("found device named '%s'", in.String())
		if strings.Contains(strings.ToLower(in.String()), strings.ToLower(searchName)) {
			log.Infof("using MIDI device %s", in.String())
			d := &Device{
				DeviceName:  in.String(),
				driver:      drv,
				device:      in,
				messageChan: make(chan [2]int, 250),
			}
			go d.handleMIDIMessageLoop()
			return d
		}
	}

	log.Errorf("unable to find MIDI device containing '%s'", searchName)
	drv.Close()
	return nil
}
