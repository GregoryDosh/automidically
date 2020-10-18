package mixer

import (
	"github.com/moutend/go-wca/pkg/wca"
)

type Device struct {
	mmd *wca.IMMDevice
	aev *wca.IAudioEndpointVolume
}

func (d *Device) DeviceName() (string, bool) {
	if d.mmd == nil {
		return "", false
	}
	var ps *wca.IPropertyStore
	if err := d.mmd.OpenPropertyStore(wca.STGM_READ, &ps); err != nil {
		return "", false
	}
	defer ps.Release()

	var pv wca.PROPVARIANT
	if err := ps.GetValue(&wca.PKEY_Device_FriendlyName, &pv); err != nil {
		return "", false
	}
	return pv.String(), true
}

func (d *Device) GetVolumeLevel() (float32, bool) {
	if d.mmd == nil {
		return 0, false
	}
	var v float32
	if err := d.aev.GetMasterVolumeLevelScalar(&v); err != nil {
		mxdLog.Error(err)
		return 0, false
	}
	return v, true
}

func (d *Device) SetVolumeLevel(v float32) (float32, bool) {
	if (v < 0) || (1 < v) {
		mxdLog.Warnf("invalid volume level %f", v)
		return 0, false
	}

	if err := d.aev.SetMasterVolumeLevelScalar(v, nil); err != nil {
		mxdLog.Warn(err)
		return 0, false
	}
	return d.GetVolumeLevel()
}

func (d *Device) Cleanup() {
	if d.aev != nil {
		d.aev.Release()
	}
	if d.mmd != nil {
		d.mmd.Release()
		d.mmd = nil
	}
}
