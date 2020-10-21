package mixer

import (
	"os"
	"strings"
	"sync"
	"time"
	"unsafe"

	sysmsg "github.com/GregoryDosh/automidically/internal/systray/message"
	"github.com/bep/debounce"
	ole "github.com/go-ole/go-ole"
	"github.com/mitchellh/go-ps"
	"github.com/moutend/go-wca/pkg/wca"
	"github.com/sirupsen/logrus"
)

var (
	mxLog  = logrus.WithField("module", "mixer")
	mpLog  = logrus.WithField("module", "mixer.mapping")
	mxaLog = logrus.WithField("module", "mixer.audiosession")
	mxdLog = logrus.WithField("module", "mixer.devices")
	das    = &devicesAndSessions{
		refreshDevices:  make(chan bool, 10),
		refreshSessions: make(chan bool, 10),
	}
)

type devicesAndSessions struct {
	inputDevice         *Device
	outputDevice        *Device
	sessions            []*AudioSession
	activeSession       *AudioSession
	systemSession       *AudioSession
	refreshDevices      chan bool
	refreshSessions     chan bool
	deviceLock          sync.Mutex
	sessionLock         sync.Mutex
	immDeviceEnumerator *wca.IMMDeviceEnumerator
}

func InitializeEnvironment() {
	mxLog.Trace("Enter InitializeEnvironment")
	defer mxLog.Trace("Exit InitializeEnvironment")

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
	// var debouncedDeviceStateChanged = func(pwstrDeviceId string, dwNewState uint64) error {
	// 	mxLog.Trace("detected changed default devices")
	// 	das.refreshDevices <- true
	// 	return nil
	// }

	// var debouncedDefaultDeviceChanged = func(flow wca.EDataFlow, role wca.ERole, pwstrDeviceId string) error {
	// 	mxLog.Trace("detected changed default devices")
	// 	das.refreshDevices <- true
	// 	return nil
	// }

	// if err := das.immDeviceEnumerator.RegisterEndpointNotificationCallback(wca.NewIMMNotificationClient(wca.IMMNotificationClientCallback{
	// 	OnDeviceStateChanged:   debouncedDeviceStateChanged,
	// 	OnDefaultDeviceChanged: debouncedDefaultDeviceChanged,
	// })); err != nil {
	// 	mxLog.Error(err)
	// 	os.Exit(1)
	// }

	go audioEventLoop()
	das.refreshDevices <- true
}

func audioEventLoop() {
	mxLog.Trace("Enter audioEventLoop")
	defer mxLog.Trace("Exit audioEventLoop")

	ddev := debounce.New(time.Second * 1)
	dses := debounce.New(time.Second * 1)
	dsesLong := debounce.New(time.Second * 5)
	periodicSessionRefresh := time.NewTicker(time.Second * 30)

	for {
		select {
		case <-das.refreshDevices:
			mxLog.Debug("default audio devices change detected")
			ddev(refreshHardwareDevices)
		case <-periodicSessionRefresh.C:
			mxLog.Debug("periodic audio session refresh triggered")
			das.refreshSessions <- false
		case instantRefresh := <-das.refreshSessions:
			if instantRefresh {
				mxLog.Debug("triggering audio session refresh")
				dses(refreshAudioSessions)
			} else {
				mxLog.Debug("triggering heavily throttled audio session refresh")
				dsesLong(refreshAudioSessions)
			}
		}
	}
}

func cleanupDevices() {
	mxLog.Trace("Enter cleanupDevices")
	defer mxLog.Trace("Exit cleanupDevices")
	if das.outputDevice != nil {
		das.outputDevice.Cleanup()
		das.outputDevice = nil
	}
	if das.inputDevice != nil {
		das.inputDevice.Cleanup()
		das.inputDevice = nil
	}
}

func cleanupSessions() {
	mxLog.Trace("Enter cleanupSessions")
	defer mxLog.Trace("Exit cleanupSessions")
	das.sessionLock.Lock()
	defer das.sessionLock.Unlock()
	for _, as := range das.sessions {
		as.Cleanup()
	}
	das.sessions = nil

	// These don't have any cleanup calls since they're
	// hopefully swept up in the full audio sessions above.
	if das.systemSession != nil {
		das.systemSession = nil
	}
	if das.activeSession != nil {
		das.activeSession = nil
	}
}

func refreshHardwareDevices() {
	mxLog.Trace("Enter refreshHardwareDevices")
	defer mxLog.Trace("Exit refreshHardwareDevices")

	das.deviceLock.Lock()
	defer das.deviceLock.Unlock()

	// Remove all stale and old pointers to things before creating new
	cleanupDevices()
	das.outputDevice = &Device{}
	das.inputDevice = &Device{}

	// Default Output Device
	if err := das.immDeviceEnumerator.GetDefaultAudioEndpoint(wca.ERender, wca.EConsole, &das.outputDevice.mmd); err != nil {
		mxLog.Warn("no default output device detected")
		return
	}
	if err := das.outputDevice.mmd.Activate(wca.IID_IAudioEndpointVolume, wca.CLSCTX_ALL, nil, &das.outputDevice.aev); err != nil {
		mxLog.Warnf("output device has no volume endpoint")
		return
	}
	if name, ok := das.outputDevice.DeviceName(); ok {
		mxLog.Infof("using default output device named: %s", name)
	}

	das.refreshSessions <- true

	// Default Input Device
	if err := das.immDeviceEnumerator.GetDefaultAudioEndpoint(wca.ECapture, wca.EConsole, &das.inputDevice.mmd); err != nil {
		mxLog.Warn("no default input device detected")
		return
	}
	if err := das.inputDevice.mmd.Activate(wca.IID_IAudioEndpointVolume, wca.CLSCTX_ALL, nil, &das.inputDevice.aev); err != nil {
		mxLog.Warnf("input device has no volume endpoint")
		return
	}
	if name, ok := das.inputDevice.DeviceName(); ok {
		mxLog.Infof("using default input device named: %s", name)
	}
}

func refreshAudioSessions() {
	mxLog.Trace("Enter refreshAudioSessions")
	defer mxLog.Trace("Exit refreshAudioSessions")

	das.deviceLock.Lock()
	defer das.deviceLock.Unlock()
	if das.outputDevice == nil || das.outputDevice.mmd == nil {
		mxLog.Warn("unable to refresh audio sessions")
		return
	}

	cleanupSessions()
	mxLog.Debug("refreshing audio sessions")

	var audioSessionManager2 *wca.IAudioSessionManager2
	if err := das.outputDevice.mmd.Activate(wca.IID_IAudioSessionManager2, wca.CLSCTX_ALL, nil, &audioSessionManager2); err != nil {
		mxLog.Warnf("failed to create IAudioSessionManager2: %s", err)
		return
	}
	defer audioSessionManager2.Release()

	var audioSessionEnumerator *wca.IAudioSessionEnumerator
	if err := audioSessionManager2.GetSessionEnumerator(&audioSessionEnumerator); err != nil {
		mxLog.Warnf("failed to create IAudioSessionEnumerator: %s", err)
		return
	}
	defer audioSessionEnumerator.Release()

	var audioSessionCount int
	if err := audioSessionEnumerator.GetCount(&audioSessionCount); err != nil {
		mxLog.Warnf("failed to get audio session count: %s", err)
		return
	}

	dn, _ := das.outputDevice.DeviceName()
	mxLog.Debugf("%d audio sessions detected for %s", audioSessionCount, dn)

	for i := 0; i < audioSessionCount; i++ {
		isSystemSession := false

		var audioSessionControl *wca.IAudioSessionControl
		if err := audioSessionEnumerator.GetSession(i, &audioSessionControl); err != nil {
			mxLog.Warnf("failed to get audio session control: %s", err)
			continue
		}
		defer audioSessionControl.Release()

		dispatch, err := audioSessionControl.QueryInterface(wca.IID_IAudioSessionControl2)
		if err != nil {
			mxLog.Warnf("failed to query interface: %s", err)
			continue
		}
		audioSessionControl2 := (*wca.IAudioSessionControl2)(unsafe.Pointer(dispatch))

		var processId uint32
		if err := audioSessionControl2.GetProcessId(&processId); err != nil {
			isSystemSession = true
			// This error code 0x889000D just means it's a multiprocess and non-unique.
			// Which means it's the system sounds, if that's not the case then some error occured.
			if err.(*ole.OleError).Code() != 0x889000D {
				mxLog.Warnf("failed to get process id: %s", err)
				continue
			}
		}

		dispatch, err = audioSessionControl2.QueryInterface(wca.IID_ISimpleAudioVolume)
		if err != nil {
			mxLog.Warnf("failed to get simple audio volume: %s", err)
			continue
		}
		simpleAudioVolume := (*wca.ISimpleAudioVolume)(unsafe.Pointer(dispatch))

		var processExecutable string
		if !isSystemSession {
			process, err := ps.FindProcess(int(processId))
			if err != nil {
				mxLog.Warnf("failed to find process: %s", err)
				continue
			}

			processExecutable = process.Executable()
		} else {
			processExecutable = "[SYSTEM]"
		}

		sess := &AudioSession{
			audioSessionControl2: audioSessionControl2,
			simpleAudioVolume:    simpleAudioVolume,
			ProcessExecutable:    processExecutable,
		}

		das.sessions = append(das.sessions, sess)

		if isSystemSession {
			das.systemSession = sess
		}

		mxLog.Tracef("added audioSession %s", processExecutable)
	}
}

func changeSessionVolume(filename string, volume float32) {
	das.sessionLock.Lock()
	defer das.sessionLock.Unlock()

	for _, f := range das.sessions {
		if strings.EqualFold(filename, f.ProcessExecutable) {
			if ok := f.SetVolume(volume); !ok {
				// False here for a heavily throttled refresh
				das.refreshSessions <- false
			}
		}
	}
}

func HandleSystrayMessage(msg sysmsg.Message) {
	switch msg {
	case sysmsg.SystrayRefreshDevices:
		das.refreshDevices <- true
	case sysmsg.SystrayRefreshSessions:
		das.refreshDevices <- true
	}
}
