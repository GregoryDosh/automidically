package main

import (
	"os"
	"runtime"
	"runtime/pprof"

	"github.com/getlantern/systray"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/GregoryDosh/automidically/internal/configurator"
	"github.com/GregoryDosh/automidically/internal/midi"
	"github.com/GregoryDosh/automidically/internal/mixer"
)

var (
	profileCPUFilename    string
	profileMemoryFilename string
	log                   = logrus.WithField("module", "main")
	globalConfigRefresh   = make(chan bool)
	configuratorInstance  *configurator.Instance
	midiDevice            *midi.Device
	mixerInstance         *mixer.Instance
)

func main() {
	app := &cli.App{
		Name:     "automidically",
		HelpName: "automidically",
		Usage:    "hooks MIDI device inputs to Windows System Volume(s)",
		Authors: []*cli.Author{
			{Name: "Gregory Dosh", Email: "GregoryDosh@users.noreply.github.com"},
		},
		Version: "0.1.0",
		Action:  automidicallyMain,
		Flags: []cli.Flag{
			&cli.StringFlag{
				EnvVars: []string{"CONFIG_FILENAME"},
				Name:    "config_filename",
				Aliases: []string{"c", "f"},
				Usage:   "specify the yml configuration location",
				Value:   "config.yml",
			},
			&cli.StringFlag{
				EnvVars: []string{"LOG_LEVEL"},
				Name:    "log_level",
				Aliases: []string{"l"},
				Usage:   "trace, debug, info, warn, error, fatal, panic",
				Value:   "info",
			},
			&cli.StringFlag{
				EnvVars:     []string{"PROFILE_CPU"},
				Name:        "profile_cpu",
				Aliases:     []string{"pc"},
				Hidden:      true,
				Destination: &profileCPUFilename,
			},
			&cli.StringFlag{
				EnvVars:     []string{"PROFILE_MEMORY"},
				Name:        "profile_memory",
				Aliases:     []string{"pm"},
				Hidden:      true,
				Destination: &profileMemoryFilename,
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func automidicallyMain(ctx *cli.Context) error {
	if profileCPUFilename != "" {
		f, err := os.Create(profileCPUFilename)
		if err != nil {
			log.Fatal("could not create CPU profile: ", err)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("could not start CPU profile: ", err)
		}
		defer pprof.StopCPUProfile()
		defer log.Infof("wrote CPU profile to %s", profileCPUFilename)
	}

	switch ctx.String("log_level") {
	case "trace":
		logrus.SetLevel(logrus.TraceLevel)
	case "debug":
		logrus.SetLevel(logrus.DebugLevel)
	case "info":
		logrus.SetLevel(logrus.InfoLevel)
	case "warn":
		logrus.SetLevel(logrus.WarnLevel)
	case "error":
		logrus.SetLevel(logrus.ErrorLevel)
	case "fatal":
		logrus.SetLevel(logrus.FatalLevel)
	case "panic":
		logrus.SetLevel(logrus.PanicLevel)
	default:
		logrus.SetLevel(logrus.InfoLevel)
	}

	log.WithFields(logrus.Fields{
		"version":       ctx.App.Version,
		"documentation": "https://github.com/GregoryDosh/automidically",
	}).Info()

	configuratorInstance = configurator.New(ctx.String("config_filename"))
	go configuratorEventLoop()

	connectMIDIDevice(configuratorInstance.MIDIDeviceName)

	var err error
	mixerInstance, err = mixer.New()
	if err != nil {
		log.Fatal(err)
	}

	systray.Run(systrayStart, systrayStop)
	log.Info("Exiting...")

	if profileMemoryFilename != "" {
		f, err := os.Create(profileMemoryFilename)
		if err != nil {
			log.Fatal("could not create memory profile: ", err)
		}
		defer f.Close()
		runtime.GC()
		if err := pprof.WriteHeapProfile(f); err != nil {
			log.Fatal("could not write memory profile: ", err)
		}
		log.Infof("wrote Memory profile to %s", profileMemoryFilename)
	}

	return nil
}

func configuratorEventLoop() {
	log := log.WithField("function", "configuratorEventLoop")
	updates := configuratorInstance.SubscribeToChanges()
	for {
		select {
		case <-globalConfigRefresh:
			configuratorInstance.ReadConfigFromDisk()
		case <-updates:
			log.Info("Got Update!")
			if midiDevice != nil {
				midiDevice.Cleanup()
				midiDevice = nil
			}
			connectMIDIDevice(configuratorInstance.MIDIDeviceName)

			log.Info(configuratorInstance.Mapping)
		}
	}
}

func connectMIDIDevice(name string) {
	log := log.WithField("function", "connectMIDIDevice")
	var err error
	midiDevice, err = midi.New(name)
	if err != nil {
		log.Error(err)
	}
	go func() {
		msgs, err := midiDevice.SubscribeToMessages()
		if err != nil {
			log.Error(err)
			return
		}
		for msg := range msgs {
			log.Tracef("Got message %+v", msg)
			mixerInstance.HandleMessage(&mixer.Message{
				Volume: float32(msg[1]) / 127,
			})
		}
		log.Debug("closing stale message handler")
	}()
}
