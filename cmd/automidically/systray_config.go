package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/getlantern/systray"
	"github.com/getlantern/systray/example/icon"
)

func systrayStart() {
	log := log.WithField("function", "systrayStart")

	systray.SetIcon(icon.Data)
	systray.SetTitle("AutoMIDIcally")
	systray.SetTooltip("AutoMIDIcally")

	mReloadConfig := systray.AddMenuItem("Reload Config", "Manual reload config.yml")
	mReloadDevices := systray.AddMenuItem("Reload Devices", "Manual reload hardware devices")

	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "Quit the whole app")
	mQuit.SetIcon(icon.Data)

	sigintc := make(chan os.Signal, 1)
	signal.Notify(sigintc, os.Interrupt, syscall.SIGTERM)

	go func() {
		defer systray.Quit()
		defer log.Debug("quitting systray")
		for {
			select {
			case <-mReloadConfig.ClickedCh:
				log.Debug("reload config clicked")
				globalConfigRefresh <- true
			case <-mReloadDevices.ClickedCh:
				log.Debug("reload devices clicked")
			case <-sigintc:
				return
			case <-mQuit.ClickedCh:
				return
			}
		}
	}()
}

func systrayStop() {
	log := log.WithField("function", "systrayStop")
	log.Debug("cleanup routine")
}
