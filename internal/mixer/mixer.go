package mixer

import (
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
	coInitializeEx     bool
	mmDeviceEnumerator *wca.IMMDeviceEnumerator

	defaultInput  *hardware.Device
	defaultOutput *hardware.Device
}

func New() (*Instance, error) {
	i := &Instance{}
	err := i.initialize()
	i.refreshHardwareDevices()
	return i, err
}

func (i *Instance) HandleMessage(m *Message) {
	i.defaultOutput.SetVolumeLevel(m.Volume)
}

func (i *Instance) initialize() error {
	// CoInitializeEx must be called at least once, and is usually called only once, for each thread that uses the COM library.
	// https://docs.microsoft.com/en-us/windows/win32/api/combaseapi/nf-combaseapi-coinitializeex
	if !i.coInitializeEx {
		if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
			if err.(*ole.OleError).Code() == 1 {
				log.Warn("CoInitializeEX returned S_FALSE -> Already initialized on this thread.")
			} else {
				return err
			}
		} else {
			log.Trace("coInitializeEx true")
			i.coInitializeEx = true
		}
	}

	// Enables audio clients to discover audio endpoint devices.
	// https://docs.microsoft.com/en-us/windows/win32/coreaudio/mmdevice-api
	if i.mmDeviceEnumerator == nil {
		if err := wca.CoCreateInstance(wca.CLSID_MMDeviceEnumerator, 0, wca.CLSCTX_ALL, wca.IID_IMMDeviceEnumerator, &i.mmDeviceEnumerator); err != nil {
			log.Warnf("CoCreateInstance failed to create MMDeviceEnumerator %s", err)
			return err
		}
		log.Trace("mmDeviceEnumerator set")
	}

	// Create a debounced notification callback for when default devices are changed.
	d := debounce.New(time.Second)
	defer d(func() {})

	var debouncedHardwareChangeCallback = func(flow wca.EDataFlow, role wca.ERole, pwstrDeviceId string) error {
		log.Trace("detected changed default devices")
		d(i.refreshHardwareDevices)
		return nil
	}
	if err := i.mmDeviceEnumerator.RegisterEndpointNotificationCallback(wca.NewIMMNotificationClient(wca.IMMNotificationClientCallback{
		OnDefaultDeviceChanged: debouncedHardwareChangeCallback,
	})); err != nil {
		log.Error(err)
		return err
	}

	log.Debug("initialized COM bindings")
	return nil
}

func (i *Instance) refreshHardwareDevices() {
	log.Debug("refreshing hardware devices")

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
