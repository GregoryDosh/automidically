package configurator

import (
	"io/ioutil"
	"strings"
	"sync"
	"time"

	"github.com/GregoryDosh/automidically/internal/midi"
	"github.com/GregoryDosh/automidically/internal/mixer"
	"github.com/GregoryDosh/automidically/internal/shell"
	"github.com/bep/debounce"
	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

var log = logrus.WithField("module", "configurator")

func New(filename string) *Configurator {
	log.Trace("Enter New")
	defer log.Trace("Exit New")
	c := &Configurator{
		filename:      filename,
		refreshConfig: make(chan bool, 1),
	}

	// This should only be called one time to prep the Windows COM bindings.
	mixer.InitializeEnvironment()

	go c.updateConfigFromDiskLoop()
	c.refreshConfig <- true

	return c
}

type MappingOptions struct {
	Mixer []mixer.Mapping `yaml:"mixer,omitempty"`
	Shell []shell.Mapping `yaml:"shell,omitempty"`
}

type Configurator struct {
	filename       string
	Mapping        MappingOptions `yaml:"mapping,omitempty"`
	MIDIDevice     *midi.Device
	MIDIDeviceName string `yaml:"midi_devicename"`
	refreshConfig  chan bool
	sync.Mutex
}

func (c *Configurator) updateConfigFromDiskLoop() {
	log.Trace("Enter updateConfigFromDiskLoop")
	defer log.Trace("Exit updateConfigFromDiskLoop")

	d := debounce.New(time.Second * 1)

	// Filewatch to reload config when the source on the disk changes.
	fileWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer fileWatcher.Close()
	if err := fileWatcher.Add(c.filename); err != nil {
		log.Fatalf("%s %s", c.filename, err)
	}

	// Filewatcher events, and manual refresh loop
	for {
		select {
		case <-c.refreshConfig:
			d(c.readConfigFromDiskAndInit)
		case event, ok := <-fileWatcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Write == fsnotify.Write {
				c.refreshConfig <- true
			}
		case err, ok := <-fileWatcher.Errors:
			if !ok {
				log.Error(err)
				return
			}
		}
	}
}

func (c *Configurator) readConfigFromDiskAndInit() {
	log.Trace("Enter readConfigFromDiskAndInit")
	defer log.Trace("Exit readConfigFromDiskAndInit")
	log.Infof("reading %s from disk", c.filename)

	f, err := ioutil.ReadFile(c.filename)
	if err != nil {
		log.Error(err)
		return
	}

	// Using an anonymous struct so we don't overwrite existing data
	// without locking and so that we don't lock or cleanup unnessarily
	// if it's not needed since we could have a bad config.
	newMapping := struct {
		Mapping        MappingOptions `yaml:"mapping"`
		MIDIDeviceName string         `yaml:"midi_devicename"`
	}{}
	if err := yaml.Unmarshal(f, &newMapping); err != nil {
		log.Errorf("unable to parse new config: %s", err)
		return
	}

	// Lock the configuration, do cleanup on the soon to be replaced configs
	// then replace the old with the new and call any initialization routines.
	c.Lock()
	defer c.Unlock()

	// Midi Device Cleanup & Initialiation
	if !strings.EqualFold(newMapping.MIDIDeviceName, c.MIDIDeviceName) {
		log.Trace("MIDI device name changed")
		if c.MIDIDevice != nil {
			c.MIDIDevice.Cleanup()
		}
		c.MIDIDeviceName = newMapping.MIDIDeviceName
		c.MIDIDevice = midi.New(c.MIDIDeviceName)
	}

	// Mixer
	for _, m := range c.Mapping.Mixer {
		m.Cleanup()
	}
	c.Mapping.Mixer = nil
	for _, m := range newMapping.Mapping.Mixer {
		m.Initialize()
		c.Mapping.Mixer = append(c.Mapping.Mixer, m)
	}

	// Shell
	for _, m := range c.Mapping.Shell {
		m.Cleanup()
	}
	c.Mapping.Shell = nil
	for _, m := range newMapping.Mapping.Shell {
		m.Initialize()
		c.Mapping.Shell = append(c.Mapping.Shell, m)
	}

	if c.MIDIDevice != nil {
		c.MIDIDevice.SetMessageCallback(c.midiMessageCallback)
	}

	log.Debugf("completed configuration reload")
	log.Debugf("%+v", c.Mapping)

}

func (c *Configurator) midiMessageCallback(cc int, v int) {
	go func() {
		c.Lock()
		defer c.Unlock()
		for _, m := range c.Mapping.Mixer {
			m.HandleMessage(cc, v)
		}
	}()
	go func() {
		c.Lock()
		defer c.Unlock()
		for _, m := range c.Mapping.Shell {
			m.HandleMessage(cc, v)
		}
	}()
}
