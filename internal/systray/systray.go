package systray

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/GregoryDosh/automidically/internal/icon"
	"github.com/GregoryDosh/automidically/internal/systray/message"
	"github.com/getlantern/systray"
	"github.com/sirupsen/logrus"
)

var (
	log = logrus.WithField("function", "systray")
)

func Start(messageHandler func(message.Message)) func() {

	return func() {
		log.Trace("Enter systrayStart")
		defer log.Trace("Exit systrayStart")

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
					messageHandler(message.SystrayRefreshConfig)
				case <-mReloadDevices.ClickedCh:
					messageHandler(message.SystrayRefreshDevices)
				case <-mReloadSessions.ClickedCh:
					messageHandler(message.SystrayRefreshSessions)
				case <-sigintc:
					return
				case <-mQuit.ClickedCh:
					return
				}
			}
		}()
	}

}

func Stop() func() {
	return func() {
		log.Trace("Enter systrayStop")
		defer log.Trace("Exit systrayStop")
	}
}