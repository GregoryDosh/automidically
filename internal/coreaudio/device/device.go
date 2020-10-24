package device

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/GregoryDosh/automidically/internal/coreaudio/audiosession"
	"github.com/moutend/go-wca/pkg/wca"
	"github.com/sirupsen/logrus"
)

var (
	log                        = logrus.WithField("module", "coreaudio.device")
	UninitializedDeviceError   = errors.New("IMMDevice is nil or uninitialized")
	MissingAudioEndpointVolume = errors.New("device has no volume endpoint")
	AudioSessionNotFound       = errors.New("audio session not found")
)

type Device struct {
	mmd           *wca.IMMDevice
	aev           *wca.IAudioEndpointVolume
	audioSessions []*audiosession.AudioSession
	sync.Mutex
}

// Cleanup will release and remove any pointers or leftover devices from the creation process.
func (d *Device) Cleanup() error {
	for _, as := range d.audioSessions {
		if err := as.Cleanup(); err != nil {
			return err
		}
	}
	if d.aev != nil {
		d.aev.Release()
		d.aev = nil
	}
	if d.mmd != nil {
		d.mmd.Release()
		d.mmd = nil
	}
	return nil
}

// DeviceName returns the name of the audio device if it exists.
func (d *Device) DeviceName() (string, error) {
	if d.mmd == nil {
		return "", UninitializedDeviceError
	}
	var ps *wca.IPropertyStore
	if err := d.mmd.OpenPropertyStore(wca.STGM_READ, &ps); err != nil {
		return "", err
	}
	defer ps.Release()

	var pv wca.PROPVARIANT
	if err := ps.GetValue(&wca.PKEY_Device_FriendlyName, &pv); err != nil {
		return "", err
	}
	return pv.String(), nil
}

// GetVolumeLevel will get the volume of the device, if it exists, as a float on the scale of 0-1.
func (d *Device) GetVolumeLevel() (float32, error) {
	if d.mmd == nil {
		return 0, UninitializedDeviceError
	}
	var v float32
	if err := d.aev.GetMasterVolumeLevelScalar(&v); err != nil {
		log.Error(err)
		return 0, err
	}
	return v, nil
}

// RefreshAudioSessions will attempt to repopulate all of the stored audio session information
// for use with the SetAudioSessionVolume, GetAudioSessionVolume, etc.
func (d *Device) RefreshAudioSessions() error {
	if d.mmd == nil {
		return UninitializedDeviceError
	}

	// Cleanup any previous audio sessions leftover from previous runs.
	for _, as := range d.audioSessions {
		if err := as.Cleanup(); err != nil {
			log.Error(err)
		}
	}
	d.audioSessions = nil

	// The IAudioSessionManager2 is for managing submixes for an audio device.
	var audioSessionManager2 *wca.IAudioSessionManager2
	if err := d.mmd.Activate(wca.IID_IAudioSessionManager2, wca.CLSCTX_ALL, nil, &audioSessionManager2); err != nil {
		return fmt.Errorf("failed to create IAudioSessionManager2: %w", err)
	}
	defer audioSessionManager2.Release()

	// The IAudioSessionEnumerator will enable the enumeration of the audio sessions
	var audioSessionEnumerator *wca.IAudioSessionEnumerator
	if err := audioSessionManager2.GetSessionEnumerator(&audioSessionEnumerator); err != nil {
		return fmt.Errorf("failed to create IAudioSessionEnumerator: %w", err)
	}
	defer audioSessionEnumerator.Release()

	var audioSessionCount int
	if err := audioSessionEnumerator.GetCount(&audioSessionCount); err != nil {
		return fmt.Errorf("failed to get audio session count: %s", err)
	}

	if dn, err := d.DeviceName(); err == nil {
		log.Debugf("%d audio sessions detected for %s", audioSessionCount, dn)
	}

	for i := 0; i < audioSessionCount; i++ {
		as, err := audiosession.New(audioSessionEnumerator, i)
		if err != nil {
			return err
		}

		d.audioSessions = append(d.audioSessions, as)

		log.Tracef("discovered audioSession %s", as.ProcessExecutable)
	}

	return nil
}

// SetVolumeLevel takes a float between 0-1 and it will set the volume of the device to that value.
func (d *Device) SetVolumeLevel(v float32) error {
	if (v < 0) || (1 < v) {
		return fmt.Errorf("invalid volume level %f", v)
	}

	if err := d.aev.SetMasterVolumeLevelScalar(v, nil); err != nil {
		return err
	}
	return nil
}

// SetAudioSessionVolumeLevel takes the sessionName which is the string to match on the ProcessExecutable of the sessions and a float between 0-1 to set the volume of any matching sessions.
func (d *Device) SetAudioSessionVolumeLevel(sessionName string, v float32) error {
	foundSession := false

	for _, f := range d.audioSessions {
		if strings.EqualFold(sessionName, f.ProcessExecutable) {
			if err := f.SetVolumeLevel(v); err != nil {
				return err
			}
			foundSession = true
		}
	}

	if !foundSession {
		return fmt.Errorf("%w: %s", AudioSessionNotFound, sessionName)
	}
	return nil
}

// New takes in a *wca.IMMDevice and wraps it as a *Device with some nice helper methods to do common tasks like SetVolumeLevel, GetVolumeLevel, etc.
func New(mmd *wca.IMMDevice) (*Device, error) {
	if mmd == nil {
		return nil, UninitializedDeviceError
	}

	d := &Device{
		mmd: mmd,
	}

	if err := d.mmd.Activate(wca.IID_IAudioEndpointVolume, wca.CLSCTX_ALL, nil, &d.aev); err != nil {
		return nil, MissingAudioEndpointVolume
	}

	return d, nil
}
