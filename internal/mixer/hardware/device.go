package hardware

import (
	"fmt"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/mitchellh/go-ps"
	"github.com/moutend/go-wca/pkg/wca"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField("module", "mixer.device")

type AudioSession struct {
	ProcessExecutable    string
	audioSessionControl2 *wca.IAudioSessionControl2
	simpleAudioVolume    *wca.ISimpleAudioVolume
}

func (a *AudioSession) Cleanup() {
	if a.simpleAudioVolume != nil {
		a.simpleAudioVolume.Release()
	}
}

func (a *AudioSession) SetVolume(v float32) bool {
	if (v < 0) || (1 < v) {
		log.Warnf("invalid volume level %f", v)
		return false
	}

	if err := a.simpleAudioVolume.SetMasterVolume(v, nil); err != nil {
		if err.(*ole.OleError).Code() == 0x88890004 { // AUDCLNT_E_DEVICE_INVALIDATED
			log.Debugf("audio session %s unavailable", a.ProcessExecutable)
			return false
		}
		log.Errorf("error setting volume: %v", err)
		return false
	}

	// Check if AudioSession is still active
	var s uint32
	if err := a.audioSessionControl2.GetState(&s); err != nil {
		log.Errorf("error getting volume state: %v", err)
		return false
	}

	if s == wca.AudioSessionStateExpired {
		log.Debug("audio session state expired")
		return false
	}

	return true
}

type Device struct {
	AudioSessions []*AudioSession
	mmd           *wca.IMMDevice
	aev           *wca.IAudioEndpointVolume
	deviceName    string
	isOutput      bool
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

func (d *Device) refreshAudioSessions() {
	for _, as := range d.AudioSessions {
		as.Cleanup()
	}
	d.AudioSessions = nil

	log.Trace("finding output audio sessions")

	var audioSessionManager2 *wca.IAudioSessionManager2
	if err := d.mmd.Activate(wca.IID_IAudioSessionManager2, wca.CLSCTX_ALL, nil, &audioSessionManager2); err != nil {
		log.Warnf("failed to create IAudioSessionManager2: %v", err)
		return
	}
	defer audioSessionManager2.Release()

	var audioSessionEnumerator *wca.IAudioSessionEnumerator
	if err := audioSessionManager2.GetSessionEnumerator(&audioSessionEnumerator); err != nil {
		log.Warnf("failed to create IAudioSessionEnumerator: %v", err)
		return
	}
	defer audioSessionEnumerator.Release()

	var audioSessionCount int
	if err := audioSessionEnumerator.GetCount(&audioSessionCount); err != nil {
		log.Warnf("failed to get audio session count: %v", err)
		return
	}
	log.Debugf("%d audio sessions detected", audioSessionCount)

	for i := 0; i < audioSessionCount; i++ {
		var audioSessionControl *wca.IAudioSessionControl
		if err := audioSessionEnumerator.GetSession(i, &audioSessionControl); err != nil {
			log.Warnf("failed to get audio session control: %v", err)
			continue
		}
		defer audioSessionControl.Release()

		dispatch, err := audioSessionControl.QueryInterface(wca.IID_IAudioSessionControl2)
		if err != nil {
			log.Warnf("failed to query interface: %v", err)
			continue
		}
		audioSessionControl2 := (*wca.IAudioSessionControl2)(unsafe.Pointer(dispatch))

		var processId uint32
		if err := audioSessionControl2.GetProcessId(&processId); err != nil {
			// This error code 0x889000D just means it's a multiprocess and non-unique.
			// Which means it's the system sounds, if that's not the case then some error occured.
			if err.(*ole.OleError).Code() != 0x889000D {
				log.Warnf("failed to get process id: %v", err)
				continue
			}
		}

		dispatch, err = audioSessionControl2.QueryInterface(wca.IID_ISimpleAudioVolume)
		if err != nil {
			log.Warnf("failed to get simple audio volume: %v", err)
			continue
		}
		simpleAudioVolume := (*wca.ISimpleAudioVolume)(unsafe.Pointer(dispatch))

		process, err := ps.FindProcess(int(processId))
		if err != nil {
			log.Warnf("failed to find process: %v", err)
			continue
		}

		d.AudioSessions = append(d.AudioSessions, &AudioSession{
			audioSessionControl2: audioSessionControl2,
			simpleAudioVolume:    simpleAudioVolume,
			ProcessExecutable:    process.Executable(),
		})

		log.Tracef("added audioSession %s", process.Executable())
	}

}

func new(imde *wca.IMMDeviceEnumerator, isOutput bool) (*Device, error) {
	d := &Device{
		isOutput: isOutput,
	}

	var deviceNumber uint32
	var dtype string
	if d.isOutput {
		deviceNumber = wca.ERender
		dtype = "output"
	} else {
		deviceNumber = wca.ECapture
		dtype = "input"
	}

	if err := imde.GetDefaultAudioEndpoint(deviceNumber, wca.EConsole, &d.mmd); err != nil {
		return d, fmt.Errorf("no default %s device detected.", dtype)
	}

	var name string
	if name, ok := d.DeviceName(); ok {
		log.Debugf("%s device %s", dtype, name)
	}
	if err := d.mmd.Activate(wca.IID_IAudioEndpointVolume, wca.CLSCTX_ALL, nil, &d.aev); err != nil {
		log.Warnf("device %s has no volume endpoint", name)
	}

	if d.isOutput {
		d.refreshAudioSessions()
	}

	return d, nil
}

func NewOutput(imde *wca.IMMDeviceEnumerator) (*Device, error) {
	return new(imde, true)
}

func NewInput(imde *wca.IMMDeviceEnumerator) (*Device, error) {
	return new(imde, false)
}
