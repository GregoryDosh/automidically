package hardware

import (
	"fmt"

	"github.com/moutend/go-wca/pkg/wca"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField("module", "mixer.device")

type Device struct {
	mmd        *wca.IMMDevice
	aev        *wca.IAudioEndpointVolume
	deviceName string
}

func (d *Device) DeviceName() (string, bool) {
	if d.mmd == nil {
		return "", false
	}
	if d.deviceName == "" {
		var ps *wca.IPropertyStore
		if err := d.mmd.OpenPropertyStore(wca.STGM_READ, &ps); err != nil {
			return "", false
		}
		defer ps.Release()

		var pv wca.PROPVARIANT
		if err := ps.GetValue(&wca.PKEY_Device_FriendlyName, &pv); err != nil {
			return "", false
		}
		d.deviceName = pv.String()
	}
	return d.deviceName, true
}

func (d *Device) GetVolumeLevel() (float32, bool) {
	if d.mmd == nil {
		return 0, false
	}
	var v float32
	if err := d.aev.GetMasterVolumeLevelScalar(&v); err != nil {
		log.Error(err)
		return 0, false
	}
	return v, true
}

func (d *Device) SetVolumeLevel(v float32) (float32, bool) {
	if (v < 0) || (1 < v) {
		log.Warnf("invalid volume level %f", v)
		return 0, false
	}

	if err := d.aev.SetMasterVolumeLevelScalar(v, nil); err != nil {
		log.Warn(err)
		return 0, false
	}
	return d.GetVolumeLevel()
}

func (d *Device) Cleanup() {
	log.Tracef("cleaning up %s", d.deviceName)
	d.deviceName = ""
	if d.aev != nil {
		d.aev.Release()
	}
	if d.mmd != nil {
		d.mmd.Release()
		d.mmd = nil
	}
}

func NewInput(imde *wca.IMMDeviceEnumerator) (*Device, error) {
	return new(imde, wca.ECapture, "input")
}

func NewOutput(imde *wca.IMMDeviceEnumerator) (*Device, error) {
	return new(imde, wca.ERender, "output")
}

func new(imde *wca.IMMDeviceEnumerator, deviceNumber uint32, dtype string) (*Device, error) {
	d := &Device{}
	if err := imde.GetDefaultAudioEndpoint(deviceNumber, wca.EConsole, &d.mmd); err != nil {
		return d, fmt.Errorf("No default %s device detected.", dtype)
	}

	var name string
	if name, ok := d.DeviceName(); ok {
		log.Debugf("input device %s", name)
	}
	if err := d.mmd.Activate(wca.IID_IAudioEndpointVolume, wca.CLSCTX_ALL, nil, &d.aev); err != nil {
		log.Warnf("device %s has no volume endpoint", name)
	}

	return d, nil
}
