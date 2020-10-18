package mixer

import (
	"strings"
	"time"

	"github.com/GregoryDosh/automidically/internal/mixer/hardware"
	"github.com/bep/debounce"
	ole "github.com/go-ole/go-ole"
	"github.com/moutend/go-wca/pkg/wca"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField("module", "mixer")

type Message struct {
	Channel string
	Volume  float32
}

type Instance struct {
	mmDeviceEnumerator *wca.IMMDeviceEnumerator

	defaultInput   *hardware.Device
	defaultOutput  *hardware.Device
	refreshDevices chan bool
}

func New() *Instance {
	i := &Instance{
		refreshDevices: make(chan bool, 10),
	}
	err := i.initialize()
	if err != nil {
		log.Fatal(err)
	}
	go i.refreshDeviceLoop()
	i.refreshDevices <- true
	return i
}

func (i *Instance) HandleMessage(m *Message) {
	for _, as := range i.defaultOutput.AudioSessions {
		if strings.Contains(strings.ToLower(as.ProcessExecutable), "discord") {
			if ok := as.SetVolume(m.Volume); !ok {
				log.Debug("audio sessions stale")
				i.refreshDevices <- true
			}
		}
	}

}

func (i *Instance) initialize() error {
	// CoInitializeEx must be called at least once, and is usually called only once, for each thread that uses the COM library.
	// https://docs.microsoft.com/en-us/windows/win32/api/combaseapi/nf-combaseapi-coinitializeex
	if err := ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED); err != nil {
		if err.(*ole.OleError).Code() == 1 {
			log.Fatalf("CoInitializeEX returned S_FALSE -> Already initialized on this thread.")
		} else {
			return err
		}
	}

	// Enables audio clients to discover audio endpoint devices.
	// https://docs.microsoft.com/en-us/windows/win32/coreaudio/mmdevice-api
	if err := wca.CoCreateInstance(wca.CLSID_MMDeviceEnumerator, 0, wca.CLSCTX_ALL, wca.IID_IMMDeviceEnumerator, &i.mmDeviceEnumerator); err != nil {
		log.Fatalf("CoCreateInstance failed to create MMDeviceEnumerator %s", err)
		return err
	}

	// Create a debounced notification callback for when default devices are changed.
	var debouncedDeviceStateChanged = func(pwstrDeviceId string, dwNewState uint64) error {
		log.Trace("detected changed default devices")
		i.refreshDevices <- true
		return nil
	}

	var debouncedDefaultDeviceChanged = func(flow wca.EDataFlow, role wca.ERole, pwstrDeviceId string) error {
		log.Trace("detected changed default devices")
		i.refreshDevices <- true
		return nil
	}

	if err := i.mmDeviceEnumerator.RegisterEndpointNotificationCallback(wca.NewIMMNotificationClient(wca.IMMNotificationClientCallback{
		OnDeviceStateChanged:   debouncedDeviceStateChanged,
		OnDefaultDeviceChanged: debouncedDefaultDeviceChanged,
	})); err != nil {
		log.Error(err)
		return err
	}

	log.Debug("initialized COM bindings")
	return nil
}

func (i *Instance) refreshDeviceLoop() {
	log.Debug("starting refreshDeviceLoop")
	defer log.Debug("stopping refreshDeviceLoop")
	d := debounce.New(time.Second * 1)
	for range i.refreshDevices {
		d(i.refreshHardwareDevices)
	}
}

func (i *Instance) refreshHardwareDevices() {
	log.Info("refreshing hardware devices")

	// Cleanup previous devices
	if i.defaultOutput != nil {
		log.Trace("cleanup defaultOutput")
		i.defaultOutput.Cleanup()
		i.defaultOutput = nil
	}
	if i.defaultInput != nil {
		log.Trace("cleanup defaultInput")
		i.defaultInput.Cleanup()
		i.defaultInput = nil
	}

	// Find New Devices
	i.defaultOutput, _ = hardware.NewOutput(i.mmDeviceEnumerator)
	i.defaultInput, _ = hardware.NewInput(i.mmDeviceEnumerator)

}
