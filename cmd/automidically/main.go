package main

import (
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"syscall"

	"github.com/GregoryDosh/automidically/internal/configurator"
	"github.com/GregoryDosh/automidically/internal/icon"
	"github.com/GregoryDosh/automidically/internal/toaster"
	"github.com/getlantern/systray"
	"github.com/orandin/lumberjackrus"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var (
	buildVersion          = "0.2.0"
	defaultLogFilename    = ""
	profileCPUFilename    string
	profileMemoryFilename string
	log                   = logrus.WithField("module", "main")
	sLog                  = log.WithField("function", "systrayStart")
)

func main() {

	app := &cli.App{
		Name:     "automidically",
		HelpName: "automidically",
		Usage:    "hooks MIDI device inputs to Windows System Volume(s)",
		Authors: []*cli.Author{
			{Name: "Gregory Dosh", Email: "GregoryDosh@users.noreply.github.com"},
		},
		Version: buildVersion,
		Action:  automidicallyMain,
		Flags: []cli.Flag{
			&cli.StringFlag{
				EnvVars: []string{"CONFIG_FILENAME"},
				Name:    "config",
				Aliases: []string{"c", "f", "config_filename", "filename"},
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
				EnvVars: []string{"LOG_PATH"},
				Name:    "log_path",
				Usage:   "Set a path for the log file. Set empty to disable.",
				Value:   defaultLogFilename,
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
		log.Fatalf("%s here", err)
	}
}

func automidicallyMain(ctx *cli.Context) error {
	var ll logrus.Level
	switch ctx.String("log_level") {
	case "trace", "t":
		ll = logrus.TraceLevel
	case "debug", "d":
		ll = logrus.DebugLevel
	case "info", "i":
		ll = logrus.InfoLevel
	case "warn", "w":
		ll = logrus.WarnLevel
	case "error", "e":
		ll = logrus.ErrorLevel
	case "fatal", "f":
		ll = logrus.FatalLevel
	case "panic", "p":
		ll = logrus.PanicLevel
	default:
		ll = logrus.InfoLevel
	}
	logrus.SetLevel(ll)

	toast := toaster.New(logrus.WarnLevel, &logrus.JSONFormatter{})
	logrus.AddHook(toast)

	log_path := ctx.String("log_path")
	if log_path != "" {
		opts := &lumberjackrus.LogFile{
			Filename:   log_path,
			MaxSize:    10,
			MaxBackups: 2,
		}
		hook, err := lumberjackrus.NewHook(opts, ll, &logrus.JSONFormatter{}, nil)
		if err != nil {
			log.Fatalf("lumber %s", err)
		}
		logrus.AddHook(hook)
	}

	log.Trace("Enter automidicallyMain")
	defer log.Trace("Exit automidicallyMain")
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

	log.WithFields(logrus.Fields{
		"version":       ctx.App.Version,
		"documentation": "https://github.com/GregoryDosh/automidically",
	}).Info()

	configurator.New(ctx.String("config"))

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
		log.Infof("wrote memory profile to %s", profileMemoryFilename)
	}

	return nil
}

func systrayStart() {
	sLog.Trace("Enter systrayStart")
	defer sLog.Trace("Exit systrayStart")

	systray.SetIcon(icon.Main)
	systray.SetTitle("AutoMIDIcally")
	systray.SetTooltip("AutoMIDIcally")

	mReload := systray.AddMenuItem("Reload", "Manual Reload")
	mReloadConfig := mReload.AddSubMenuItem("Config", "Manual reload config.yml")
	mReloadDevices := mReload.AddSubMenuItem("Devices", "Manual reload hardware devices")
	mReloadSessions := mReload.AddSubMenuItem("Sessions", "Manual reload audio sessions")

	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "Quit AutoMIDIcally")

	sigintc := make(chan os.Signal, 1)
	signal.Notify(sigintc, os.Interrupt, syscall.SIGTERM)

	go func() {
		defer systray.Quit()
		defer log.Debug("quitting systray")
		for {
			select {
			case <-mReloadConfig.ClickedCh:
				sLog.Debug("reload config clicked")
			case <-mReloadDevices.ClickedCh:
				sLog.Debug("reload devices clicked")
			case <-mReloadSessions.ClickedCh:
				sLog.Debug("reload sessions clicked")
			case <-sigintc:
				return
			case <-mQuit.ClickedCh:
				return
			}
		}
	}()
}

func systrayStop() {
	sLog.Trace("Enter systrayStop")
	defer sLog.Trace("Exit systrayStop")
}
