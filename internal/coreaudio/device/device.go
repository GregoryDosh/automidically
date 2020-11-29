package device

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/GregoryDosh/automidically/internal/coreaudio/audiosession"
	"github.com/bep/debounce"
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
	mmd                  *wca.IMMDevice
	aev                  *wca.IAudioEndpointVolume
	audioSessions        []*audiosession.AudioSession
	audioSessionManager2 *wca.IAudioSessionManager2
	sessionNotification  *wca.IAudioSessionNotification
	sync.Mutex
}

// Cleanup will release and remove any pointers or leftover devices from the creation process.
func (d *Device) Cleanup() error {
	d.Lock()
	defer d.Unlock()

	if d.audioSessionManager2 != nil {
		if err := d.audioSessionManager2.UnregisterSessionNotification(d.sessionNotification); err != nil {
			log.Error(err)
		}
		d.audioSessionManager2.Release()
	}
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

// createDebouncedOnSessionCreateFunction is called when there is a new audio session created on this device.
// This looks really weird because the onSessionCreated function can get called many times
// in rapid succession so we want to debounce that function call. But the callback has to take
// a very specific format, so we wrap it up. On top of that the debounce function also wants a particular
// signature, so it needs to get wrapped up too.
func (d *Device) createDebouncedOnSessionCreateFunction() func(*wca.IAudioSessionControl) (hResult uintptr) {
	deb := debounce.New(time.Second * 1)
	wrappedFunc := func() {
		log.Trace("detected onSessionCreate event")
		if err := d.RefreshAudioSessions(); err != nil {
			if !errors.Is(err, UninitializedDeviceError) {
				log.Error(err)
			}
		}
	}
	return func(s *wca.IAudioSessionControl) (hResult uintptr) {
		deb(wrappedFunc)
		return
	}
}

// RefreshAudioSessions will attempt to repopulate all of the stored audio session information
// for use with the SetAudioSessionVolume, GetAudioSessionVolume, etc.
func (d *Device) RefreshAudioSessions() error {
	d.Lock()
	defer d.Unlock()
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
	if err := d.mmd.Activate(wca.IID_IAudioSessionManager2, wca.CLSCTX_ALL, nil, &d.audioSessionManager2); err != nil {
		return fmt.Errorf("failed to create IAudioSessionManager2: %w", err)
	}

	// This will let us be notified of new audio sessions happening on the device so we can rescan
	if d.sessionNotification == nil {
		debouncedOnSessionCreate := d.createDebouncedOnSessionCreateFunction()
		noopCallback := func(sess *wca.IAudioSessionControl) (hResult uintptr) {
			return
		}
		d.sessionNotification = &wca.IAudioSessionNotification{
			VTable: &wca.IAudioSessionNotificationVtbl{
				OnSessionCreated: syscall.NewCallback(debouncedOnSessionCreate),
				AddRef:           syscall.NewCallback(noopCallback),
				Release:          syscall.NewCallback(noopCallback),
				QueryInterface:   syscall.NewCallback(noopCallback),
			},
		}
	}
	if err := d.audioSessionManager2.RegisterSessionNotification(d.sessionNotification); err != nil {
		log.Error(err)
	}

	// The IAudioSessionEnumerator will enable the enumeration of the audio sessions
	var audioSessionEnumerator *wca.IAudioSessionEnumerator
	if err := d.audioSessionManager2.GetSessionEnumerator(&audioSessionEnumerator); err != nil {
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
	d.Lock()
	defer d.Unlock()
	foundSession := false

	for _, f := range d.audioSessions {
		if strings.EqualFold(sessionName, f.ProcessExecutable) {
			if err := f.SetVolumeLevel(v); err != nil {
				if !errors.Is(err, audiosession.ErrorAudioSessionStateExpired) {
					return err
				}
				continue
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
