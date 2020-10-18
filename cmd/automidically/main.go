package main

import (
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"syscall"

	"github.com/GregoryDosh/automidically/internal/configurator"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var (
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
		Version: "0.2.0",
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
	switch ctx.String("log_level") {
	case "trace", "t":
		logrus.SetLevel(logrus.TraceLevel)
	case "debug", "d":
		logrus.SetLevel(logrus.DebugLevel)
	case "info", "i":
		logrus.SetLevel(logrus.InfoLevel)
	case "warn", "w":
		logrus.SetLevel(logrus.WarnLevel)
	case "error", "e":
		logrus.SetLevel(logrus.ErrorLevel)
	case "fatal", "f":
		logrus.SetLevel(logrus.FatalLevel)
	case "panic", "p":
		logrus.SetLevel(logrus.PanicLevel)
	default:
		logrus.SetLevel(logrus.InfoLevel)
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

	sigintc := make(chan os.Signal, 1)
	signal.Notify(sigintc, os.Interrupt, syscall.SIGTERM)

	<-sigintc
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
