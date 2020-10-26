package configurator

import (
	"io/ioutil"
	"strings"
	"sync"
	"time"

	"github.com/GregoryDosh/automidically/internal/coreaudio"
	"github.com/GregoryDosh/automidically/internal/midi"
	"github.com/GregoryDosh/automidically/internal/mixer"
	"github.com/GregoryDosh/automidically/internal/shell"
	"github.com/GregoryDosh/automidically/internal/systray"
	"github.com/bep/debounce"
	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

var log = logrus.WithField("module", "configurator")

type MappingOptions struct {
	Mixer []mixer.Mapping `yaml:"mixer,omitempty"`
	Shell []shell.Mapping `yaml:"shell,omitempty"`
}

type Configurator struct {
	filename       string
	EchoMIDIEvents bool           `yaml:"echoMIDIEvents"`
	Mapping        MappingOptions `yaml:"mapping,omitempty"`
	MIDIDevice     *midi.Device
	MIDIDeviceName string `yaml:"midiDevicename"`
	coreAudio      *coreaudio.CoreAudio
	reloadConfig   chan bool
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
		case <-c.reloadConfig:
			d(c.readConfigFromDiskAndInit)
		case event, ok := <-fileWatcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Write == fsnotify.Write {
				c.reloadConfig <- true
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
		MIDIDeviceName string         `yaml:"midiDevicename"`
		EchoMIDIEvents bool           `yaml:"echoMIDIEvents"`
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
		if c.MIDIDeviceName != "" {
			log.Trace("MIDI device name changed")
		}
		if c.MIDIDevice != nil {
			if err := c.MIDIDevice.Cleanup(); err != nil {
				log.Error(err)
			}
		}
		c.MIDIDeviceName = newMapping.MIDIDeviceName
		c.MIDIDevice = midi.New(c.MIDIDeviceName)
	}

	// Mixer
	for _, mapping := range newMapping.Mapping.Mixer {
		if err := mapping.Validate(); err != nil {
			log.Errorf("unable to parse new config: %s", err)
			return
		}
	}
	c.Mapping.Mixer = newMapping.Mapping.Mixer

	// Shell
	c.Mapping.Shell = newMapping.Mapping.Shell

	if c.MIDIDevice != nil {
		c.MIDIDevice.SetMessageCallback(c.midiMessageCallback)
	}

	// EchoMIDIEvents
	c.EchoMIDIEvents = newMapping.EchoMIDIEvents

	log.Debug("completed configuration reload")
	log.Tracef("%+v", c.Mapping)
}

func (c *Configurator) midiMessageCallback(cc int, v int) {
	c.Lock()
	defer c.Unlock()
	if c.EchoMIDIEvents {
		log.Infof("CC: %d, Value: %d", cc, v)
	}
	for _, m := range c.Mapping.Mixer {
		go func(m mixer.Mapping) {
			c.coreAudio.HandleMIDIMessage(&m, cc, v)
		}(m)
	}
	for _, m := range c.Mapping.Shell {
		go func(m shell.Mapping) {
			m.HandleMIDIMessage(cc, v)
		}(m)
	}
}

func (c *Configurator) HandleSystrayMessage(msg systray.Message) {
	if msg == systray.SystrayRefreshConfig {
		c.reloadConfig <- true
		return
	}
	if msg == systray.SystrayQuit {
		log.Trace("Starting cleanup & shutdown procedures.")
		if err := c.MIDIDevice.Cleanup(); err != nil {
			log.Error(err)
		}
		if err := c.coreAudio.Cleanup(); err != nil {
			log.Error(err)
		}
		return
	}
	go func() {
		c.Lock()
		defer c.Unlock()
		c.coreAudio.HandleSystrayMessage(msg)
	}()
	go func() {
		c.Lock()
		defer c.Unlock()
		shell.HandleSystrayMessage(msg)
	}()
}

func New(filename string) *Configurator {
	ca, err := coreaudio.New()
	if err != nil {
		log.Error(err)
	}

	c := &Configurator{
		filename:     filename,
		reloadConfig: make(chan bool, 1),
		coreAudio:    ca,
	}

	go c.updateConfigFromDiskLoop()
	c.reloadConfig <- true

	return c
}
