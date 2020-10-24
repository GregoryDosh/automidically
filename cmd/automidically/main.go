package main

import (
	"os"
	"runtime"
	"runtime/pprof"

	"github.com/GregoryDosh/automidically/internal/configurator"
	tray "github.com/GregoryDosh/automidically/internal/systray"
	"github.com/GregoryDosh/automidically/internal/toaster"
	"github.com/getlantern/systray"
	"github.com/orandin/lumberjackrus"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var (
	buildVersion          = "0.2.1"
	defaultLogFilename    = ""
	configFilename        = ""
	profileCPUFilename    string
	profileMemoryFilename string
	log                   = logrus.WithField("module", "main")
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
				EnvVars:     []string{"CONFIG_FILENAME"},
				Name:        "config",
				Aliases:     []string{"c", "f", "config_filename", "filename"},
				Usage:       "specify the yml configuration location",
				Destination: &configFilename,
				Value:       "config.yml",
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
			&cli.BoolFlag{
				EnvVars: []string{"NOTIFICATIONS"},
				Aliases: []string{"n"},
				Name:    "notifications",
				Usage:   "Enables Windows 10 Notifications",
				Value:   false,
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

	if ctx.Bool("notifications") {
		toast := toaster.New(logrus.WarnLevel, &logrus.JSONFormatter{})
		logrus.AddHook(toast)
	}

	log_path := ctx.String("log_path")
	if log_path != "" {
		opts := &lumberjackrus.LogFile{
			Filename:   log_path,
			MaxSize:    10,
			MaxBackups: 2,
		}
		hook, err := lumberjackrus.NewHook(opts, ll, &logrus.JSONFormatter{}, nil)
		if err != nil {
			log.Fatal(err)
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

	c := configurator.New(configFilename)
	systray.Run(tray.Start(c.HandleSystrayMessage), tray.Stop())
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
