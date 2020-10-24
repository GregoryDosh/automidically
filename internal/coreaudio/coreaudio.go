package coreaudio

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/GregoryDosh/automidically/internal/coreaudio/audiosession"
	"github.com/GregoryDosh/automidically/internal/coreaudio/device"
	"github.com/GregoryDosh/automidically/internal/mixer"
	"github.com/GregoryDosh/automidically/internal/systray"
	"github.com/bep/debounce"
	ole "github.com/go-ole/go-ole"
	"github.com/moutend/go-wca/pkg/wca"
	"github.com/sirupsen/logrus"
)

var (
	CoreAudioAlreadyInitialized = errors.New("CoInitializeEX returned S_FALSE -> Already initialized on this thread")
	log                         = logrus.WithField("module", "coreaudio")
)

type CoreAudio struct {
	inputDevice                   *device.Device
	outputDevice                  *device.Device
	refreshHardwareDevicesChannel chan bool
	refreshAudioSessionsChannel   chan bool
	deviceLock                    sync.Mutex
	deviceEnumerator              *wca.IMMDeviceEnumerator
	notificationClient            *wca.IMMNotificationClient
	cleanupChan                   chan bool
}

// cleanupDevices is an internal function to do some more of the grunt work around the device cleanup process.
// This might get called multiple times when refreshing devices during the event looping.
func (ca *CoreAudio) cleanupDevices() {
	log.Trace("Enter cleanupDevices")
	defer log.Trace("Exit cleanupDevices")

	ca.deviceLock.Lock()
	defer ca.deviceLock.Unlock()

	if ca.outputDevice != nil {
		if err := ca.outputDevice.Cleanup(); err != nil {
			log.Error(err)
		}
		ca.outputDevice = nil
	}
	if ca.inputDevice != nil {
		if err := ca.inputDevice.Cleanup(); err != nil {
			log.Error(err)
		}
		ca.inputDevice = nil
	}
}

// coreAudioEventLoop is reponsible for the coordination of the logic in the coreaudio package. It will handle events
// on the different channels and call out to different routines as necessary.
func (ca *CoreAudio) coreAudioEventLoop() {
	log.Trace("Enter coreAudioEventLoop")
	defer log.Trace("Exit coreAudioEventLoop")

	ddev := debounce.New(time.Second * 1)
	dses := debounce.New(time.Second * 1)
	dsesLong := debounce.New(time.Second * 5)
	periodicSessionRefresh := time.NewTicker(time.Second * 30)

	for {
		select {
		case <-ca.cleanupChan:
			return
		case <-ca.refreshHardwareDevicesChannel:
			log.Debug("default audio devices change detected")
			ddev(ca.refreshHardwareDevices)
		case <-periodicSessionRefresh.C:
			log.Trace("periodic audio session refresh triggered")
			ca.refreshAudioSessionsChannel <- false
		case instantRefresh := <-ca.refreshAudioSessionsChannel:
			if instantRefresh {
				log.Trace("triggering audio session refresh")
				dses(ca.refreshAudioSessions)
			} else {
				log.Trace("triggering heavily debounced audio session refresh")
				dsesLong(ca.refreshAudioSessions)
			}
		}
	}
}

// refreshAudioSessions is the internal implementation that will walk through the devices and attempt to refresh their respective audio sessions.
// This should only get called from the core audio event loop.
func (ca *CoreAudio) refreshAudioSessions() {
	log.Trace("Enter refreshAudioSessions")
	defer log.Trace("Exit refreshAudioSessions")
	ca.deviceLock.Lock()
	defer ca.deviceLock.Unlock()
	if ca.outputDevice != nil {
		if err := ca.outputDevice.RefreshAudioSessions(); err != nil {
			log.Error(err)
		}
	}
}

// refreshHardwareDevices is the internal implementation that will try to find the new input/output devices and their associated audio sessions.
// This should only get called from the core audio event loop.
func (ca *CoreAudio) refreshHardwareDevices() {
	log.Trace("Enter refreshHardwareDevices")
	defer log.Trace("Exit refreshHardwareDevices")

	var err error

	// Remove all stale and old pointers to things before creating new
	ca.cleanupDevices()

	// Anytime we'll be changing references to devices we should lock this
	// so we don't iterate over changing slices or try to reference nil pointers etc.
	ca.deviceLock.Lock()
	defer ca.deviceLock.Unlock()

	// Default Output Device
	var outDev *wca.IMMDevice
	if err := ca.deviceEnumerator.GetDefaultAudioEndpoint(wca.ERender, wca.EConsole, &outDev); err != nil {
		log.Warn("no default output device detected")
		return
	}

	if ca.outputDevice, err = device.New(outDev); err != nil {
		if errors.Is(err, device.MissingAudioEndpointVolume) {
			log.Debug(err)
		} else {
			log.Error(err)
		}
	}
	if name, err := ca.outputDevice.DeviceName(); err != nil {
		log.Infof("using default output device named: %s", name)
	}

	// Default Input Device
	var inDev *wca.IMMDevice
	if err := ca.deviceEnumerator.GetDefaultAudioEndpoint(wca.ECapture, wca.EConsole, &inDev); err != nil {
		log.Warn("no default input device detected")
		return
	}
	if ca.inputDevice, err = device.New(inDev); err != nil {
		if errors.Is(err, device.MissingAudioEndpointVolume) {
			log.Debug(err)
		} else {
			log.Error(err)
		}
	}
	if name, err := ca.inputDevice.DeviceName(); err != nil {
		log.Infof("using default input device named: %s", name)
	}

	// Since the default devices change, refresh their audio sessions too.
	ca.refreshAudioSessionsChannel <- true
}

func (ca *CoreAudio) onDefaultDeviceChanged(flow wca.EDataFlow, role wca.ERole, pwstrDeviceId string) error {
	if flow == wca.ERender {
		log.Trace("detected onDefaultDeviceChanged event: output")
	} else if flow == wca.ECapture {
		log.Trace("detected onDefaultDeviceChanged event: input")
	} else {
		log.Trace("detected onDefaultDeviceChanged event: unknown")
	}
	ca.refreshHardwareDevicesChannel <- true
	return nil
}

func (ca *CoreAudio) onDeviceAdded(pwstrDeviceId string) error {
	log.Trace("detected onDeviceAdded event")
	ca.refreshHardwareDevicesChannel <- true
	return nil
}

func (ca *CoreAudio) onDeviceRemoved(pwstrDeviceId string) error {
	log.Trace("detected onDeviceRemoved event")
	ca.refreshHardwareDevicesChannel <- true
	return nil
}

func (ca *CoreAudio) onDeviceStateChanged(pwstrDeviceId string, dwNewState uint64) error {
	log.Trace("detected onDeviceStateChanged event")
	ca.refreshHardwareDevicesChannel <- true
	return nil
}

// Cleanup is called by other packages to close down the event loop, clear any pointers, and get ready for shutdown.
func (ca *CoreAudio) Cleanup() error {
	log.Trace("Enter Cleanup")
	defer log.Trace("Exit Cleanup")

	if ca.cleanupChan != nil {
		close(ca.cleanupChan)
	}

	ca.cleanupDevices()
	return nil
}

// HandleMIDIMessage will take a *mixer.Mapping, and the MIDI's channel c, along with the value sent v, to peform the necessary logic
// of refreshing devices, setting volumes of audio sessions, devices, and other potential scenarios.
func (ca *CoreAudio) HandleMIDIMessage(m *mixer.Mapping, c int, v int) {
	if m.Cc != c {
		return
	}

	ca.deviceLock.Lock()
	defer ca.deviceLock.Unlock()
	if ca.outputDevice == nil {
		return
	}

	clampedValue := math.Max(m.Min, math.Min(m.Max, float64(v)))
	newValue := float32(clampedValue / m.Max)
	if m.Reverse {
		newValue = 1 - newValue
	}

	// special
	for _, s := range m.Special {
		// refresh_devices
		if strings.EqualFold(s, "refresh_devices") {
			ca.refreshHardwareDevicesChannel <- true
		}
		// refresh_sessions
		if strings.EqualFold(s, "refresh_sessions") {
			ca.refreshAudioSessionsChannel <- true
		}
		// output
		if strings.EqualFold(s, "output") {
			if err := ca.outputDevice.SetVolumeLevel(newValue); err != nil {
				log.Error(err)
			}
		}
		// input
		if strings.EqualFold(s, "input") {
			if err := ca.inputDevice.SetVolumeLevel(newValue); err != nil {
				log.Error(err)
			}
		}
		// system
		if strings.EqualFold(s, "system") {
			if err := ca.outputDevice.SetAudioSessionVolumeLevel(audiosession.SystemAudioSession, newValue); err != nil {
				log.Error(err)
			}
		}
	}

	// filename
	for _, f := range m.Filename {
		if err := ca.outputDevice.SetAudioSessionVolumeLevel(f, newValue); err != nil {
			if !errors.Is(err, device.AudioSessionNotFound) {
				log.Error(err)
			}
		}
	}

	// device
	for _, d := range m.Device {
		log.Debugf("Device Control Not Implemented: %s", d)
	}
}

// HandleSystrayMessage takes messages from systray and will act accordingy.
func (ca *CoreAudio) HandleSystrayMessage(msg systray.Message) {
	switch msg {
	case systray.SystrayRefreshDevices:
		ca.refreshHardwareDevicesChannel <- true
	case systray.SystrayRefreshSessions:
		ca.refreshAudioSessionsChannel <- true
	}
}

// New will create a new CoreAudio interface and start all the event loops and bindings necessasry to inderact with Window's Audio APIs
func New() (*CoreAudio, error) {
	// CoInitializeEx must be called at least once, and is usually called only once, for each thread that uses the COM library.
	// https://docs.microsoft.com/en-us/windows/win32/api/combaseapi/nf-combaseapi-coinitializeex
	if err := ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED); err != nil {
		if err.(*ole.OleError).Code() == 1 {
			return nil, CoreAudioAlreadyInitialized

		} else {
			return nil, err
		}
	}

	ca := &CoreAudio{
		refreshHardwareDevicesChannel: make(chan bool, 20),
		refreshAudioSessionsChannel:   make(chan bool, 20),
		cleanupChan:                   make(chan bool, 1),
	}

	// Enables audio clients to discover audio endpoint devices.
	// https://docs.microsoft.com/en-us/windows/win32/coreaudio/mmdevice-api
	if err := wca.CoCreateInstance(wca.CLSID_MMDeviceEnumerator, 0, wca.CLSCTX_ALL, wca.IID_IMMDeviceEnumerator, &ca.deviceEnumerator); err != nil {
		return nil, fmt.Errorf("CoCreateInstance failed to create MMDeviceEnumerator %w", err)
	}

	// The notification client will be used to trigger refreshHardwareDevicesChannel later on important events happening like new or changed devices.
	ca.notificationClient = wca.NewIMMNotificationClient(wca.IMMNotificationClientCallback{
		OnDefaultDeviceChanged: ca.onDefaultDeviceChanged,
		OnDeviceAdded:          ca.onDeviceAdded,
		OnDeviceRemoved:        ca.onDeviceRemoved,
		OnDeviceStateChanged:   ca.onDeviceStateChanged,
	})

	if err := ca.deviceEnumerator.RegisterEndpointNotificationCallback(ca.notificationClient); err != nil {
		return nil, err
	}

	go ca.coreAudioEventLoop()
	ca.refreshHardwareDevicesChannel <- true

	return ca, nil
}
