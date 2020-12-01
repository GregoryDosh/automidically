package systray

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/GregoryDosh/automidically/internal/icon"
	"github.com/getlantern/systray"
	"github.com/lxn/walk"
	"github.com/sirupsen/logrus"
)

var (
	log            = logrus.WithField("function", "systray")
	smAudioDevices = map[string]*systray.MenuItem{}
	mAudioDevices  *systray.MenuItem
)

func Start(messageHandler func(Message)) func() {

	return func() {
		log.Trace("Enter systrayStart")
		defer log.Trace("Exit systrayStart")

		systray.SetIcon(icon.Main)
		systray.SetTitle("AutoMIDIcally")
		systray.SetTooltip("AutoMIDIcally")

		mAudioDevices = systray.AddMenuItem("Audio Devices", "List of detected audio devices.")
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
					messageHandler(SystrayRefreshConfig)
				case <-mReloadDevices.ClickedCh:
					messageHandler(SystrayRefreshDevices)
				case <-mReloadSessions.ClickedCh:
					messageHandler(SystrayRefreshSessions)
				case <-sigintc:
					messageHandler(SystrayQuit)
					return
				case <-mQuit.ClickedCh:
					messageHandler(SystrayQuit)
					return
				}
			}
		}()
	}

}

func SetAudioDevices(devices []string) {
	if mAudioDevices == nil {
		log.Error("unable to set audio devices")
		return
	}
	// This is kind of hacky, but since there isn't a way to remove menu items we have to hide them instead.
	// This is maybe not that bad for audio devices since this list shoudn't change a whole lot in cardinality
	// over time. But it still feels wrong :-\
	// https://github.com/getlantern/systray/issues/54
	for _, menuItem := range smAudioDevices {
		menuItem.Hide()
	}

	for _, name := range devices {
		if device, ok := smAudioDevices[name]; ok {
			device.Show()
			continue
		}
		menuItem := mAudioDevices.AddSubMenuItem(name, "Click to copy name to clipboard.")
		smAudioDevices[name] = menuItem

		go audioDeviceClickHandler(menuItem, name)
	}
}

func audioDeviceClickHandler(m *systray.MenuItem, name string) {
	for range m.ClickedCh {
		if err := walk.Clipboard().SetText(name); err != nil {
			log.Error(err)
		}
	}
}
