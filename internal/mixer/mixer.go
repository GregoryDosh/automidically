package mixer

import (
	"os"
	"sync"
	"time"

	"github.com/bep/debounce"
	ole "github.com/go-ole/go-ole"
	"github.com/moutend/go-wca/pkg/wca"
	"github.com/sirupsen/logrus"
)

var (
	mxLog  = logrus.WithField("module", "mixer")
	mpLog  = logrus.WithField("module", "mixer.mapping")
	mxaLog = logrus.WithField("module", "mixer.audiosession")
	mxdLog = logrus.WithField("module", "mixer.devices")
	das    = &devicesAndSessions{
		refresh: make(chan bool, 10),
	}
)

type devicesAndSessions struct {
	inputDevice         *Device
	outputDevice        *Device
	sessions            map[string]AudioSession
	immDeviceEnumerator *wca.IMMDeviceEnumerator
	refresh             chan bool
	sync.Mutex
}

func init() {
	// CoInitializeEx must be called at least once, and is usually called only once, for each thread that uses the COM library.
	// https://docs.microsoft.com/en-us/windows/win32/api/combaseapi/nf-combaseapi-coinitializeex
	if err := ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED); err != nil {
		if err.(*ole.OleError).Code() == 1 {
			mxLog.Error("CoInitializeEX returned S_FALSE -> Already initialized on this threa")
			os.Exit(1)

		} else {
			mxLog.Fatal(err)
			os.Exit(1)
		}
	}

	// Enables audio clients to discover audio endpoint devices.
	// https://docs.microsoft.com/en-us/windows/win32/coreaudio/mmdevice-api
	if err := wca.CoCreateInstance(wca.CLSID_MMDeviceEnumerator, 0, wca.CLSCTX_ALL, wca.IID_IMMDeviceEnumerator, &das.immDeviceEnumerator); err != nil {
		mxLog.Fatalf("CoCreateInstance failed to create MMDeviceEnumerator %s", err)
		os.Exit(1)
	}

	// Create a debounced notification callback for when default devices are change
	var debouncedDeviceStateChanged = func(pwstrDeviceId string, dwNewState uint64) error {
		mxLog.Trace("detected changed default devices")
		das.refresh <- true
		return nil
	}

	var debouncedDefaultDeviceChanged = func(flow wca.EDataFlow, role wca.ERole, pwstrDeviceId string) error {
		mxLog.Trace("detected changed default devices")
		das.refresh <- true
		return nil
	}

	if err := das.immDeviceEnumerator.RegisterEndpointNotificationCallback(wca.NewIMMNotificationClient(wca.IMMNotificationClientCallback{
		OnDeviceStateChanged:   debouncedDeviceStateChanged,
		OnDefaultDeviceChanged: debouncedDefaultDeviceChanged,
	})); err != nil {
		mxLog.Error(err)
		os.Exit(1)
	}

	go handleChangedDeviceLoop()
	das.refresh <- true
}

func handleChangedDeviceLoop() {
	mxdLog.Trace("Enter handleChangedDeviceLoop")
	defer mxdLog.Trace("Exit handleChangedDeviceLoop")
	d := debounce.New(time.Second * 1)
	for range das.refresh {
		mxdLog.Debug("default audio devices change detected")
		d(refreshHardwareDevices)
	}
}

func cleanupDevices() {
	mxdLog.Trace("Enter cleanupDevices")
	defer mxdLog.Trace("Exit cleanupDevices")
	if das.outputDevice != nil {
		das.outputDevice.Cleanup()
		das.outputDevice = nil
	}
	if das.inputDevice != nil {
		das.inputDevice.Cleanup()
		das.inputDevice = nil
	}
}

func refreshHardwareDevices() {
	mxdLog.Trace("Enter refreshHardwareDevices")
	defer mxdLog.Trace("Exit refreshHardwareDevices")

	das.Lock()
	defer das.Unlock()

	// Remove all stale and old pointers to things before creating new
	cleanupDevices()
	das.outputDevice = &Device{}
	das.inputDevice = &Device{}

	// Default Output Device
	if err := das.immDeviceEnumerator.GetDefaultAudioEndpoint(wca.ERender, wca.EConsole, &das.outputDevice.mmd); err != nil {
		mxdLog.Warn("no default output device detected")
		return
	}
	if err := das.outputDevice.mmd.Activate(wca.IID_IAudioEndpointVolume, wca.CLSCTX_ALL, nil, &das.outputDevice.aev); err != nil {
		mxdLog.Warnf("output device has no volume endpoint")
		return
	}
	if name, ok := das.outputDevice.DeviceName(); ok {
		mxdLog.Infof("using default output device named: %s", name)
	}

	// Default Input Device
	if err := das.immDeviceEnumerator.GetDefaultAudioEndpoint(wca.ECapture, wca.EConsole, &das.inputDevice.mmd); err != nil {
		mxdLog.Warn("no default input device detected")
		return
	}
	if err := das.inputDevice.mmd.Activate(wca.IID_IAudioEndpointVolume, wca.CLSCTX_ALL, nil, &das.inputDevice.aev); err != nil {
		mxdLog.Warnf("input device has no volume endpoint")
		return
	}
	if name, ok := das.inputDevice.DeviceName(); ok {
		mxdLog.Infof("using default input device named: %s", name)
	}
}
