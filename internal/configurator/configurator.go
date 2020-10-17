package configurator

import (
	"io/ioutil"
	"sync"
	"time"

	"github.com/bep/debounce"
	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

var log = logrus.WithField("module", "configurator")

type MappingOption struct {
	Filename string `yaml:"filename,omitempty"`
	Title    string `yaml:"title,omitempty"`
	Special  string `yaml:"special,omitempty"`
}

type Instance struct {
	configLocation string
	Mapping        map[int][]MappingOption `yaml:"mapping"`
	MIDIDeviceName string                  `yaml:"midi_devicename"`
	subscribers    []chan bool
	sync.Mutex
}

func New(filename string) *Instance {
	if filename == "" {
		log.Fatal("unable to read config, empty filename")
	}

	i := &Instance{
		configLocation: filename,
	}

	i.ReadConfigFromDisk()
	go i.configFileWatcher()

	return i
}

func (i *Instance) configFileWatcher() {
	log.Debug("starting filewatcher")
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	err = watcher.Add(i.configLocation)
	if err != nil {
		log.Fatalf("%s %s", i.configLocation, err)
	}

	d := debounce.New(time.Second)
	defer d(func() {})

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Write == fsnotify.Write {
				log.Debugf("%s modified", event.Name)
				d(i.ReadConfigFromDisk)
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Error(err)
		}
	}
}

func (i *Instance) ReadConfigFromDisk() {
	log.Infof("reading %s from disk", i.configLocation)

	f, err := ioutil.ReadFile(i.configLocation)
	if err != nil {
		log.Error(err)
		return
	}

	i.Lock()
	defer i.Unlock()
	err = yaml.Unmarshal(f, &i)
	if err != nil {
		log.Error(err)
		return
	}

	go func() {
		i.Lock()
		defer i.Unlock()
		for _, e := range i.subscribers {
			select {
			case e <- true:
			case <-time.After(time.Second):
				log.Warn("dispatching config updates timed-out")
			}
		}
	}()
}

func (i *Instance) SubscribeToChanges() chan bool {
	c := make(chan bool, 1)
	i.Lock()
	defer i.Unlock()
	i.subscribers = append(i.subscribers, c)
	return c
}
