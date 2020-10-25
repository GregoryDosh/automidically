package audiosession

import (
	"errors"
	"fmt"
	"sync"
	"unsafe"

	ole "github.com/go-ole/go-ole"
	"github.com/mitchellh/go-ps"
	"github.com/moutend/go-wca/pkg/wca"
	"github.com/sirupsen/logrus"
)

var (
	log                            = logrus.WithField("module", "coreaudio.audiosession")
	UninitializedAudioSessionError = errors.New("Audio Session uninitialized")
)

const (
	SystemAudioSession = "[System Process]"
)

type AudioSession struct {
	ProcessExecutable    string
	audioSessionControl2 *wca.IAudioSessionControl2
	simpleAudioVolume    *wca.ISimpleAudioVolume
	sync.Mutex
}

// Cleanup will release and remove any pointers or leftover devices from the creation process.
func (a *AudioSession) Cleanup() error {
	log.Tracef("cleaning up %s", a.ProcessExecutable)
	a.Lock()
	defer a.Unlock()
	if a.simpleAudioVolume != nil {
		a.simpleAudioVolume.Release()
	}
	if a.audioSessionControl2 != nil {
		a.audioSessionControl2.Release()
	}
	a.ProcessExecutable = ""
	return nil
}

// SetVolumeLevel takes a float between 0-1 and it will set the volume of the audio session to that value.
func (a *AudioSession) SetVolumeLevel(v float32) error {
	a.Lock()
	defer a.Unlock()

	if (v < 0) || (1 < v) {
		return fmt.Errorf("invalid volume level %f", v)
	}

	if err := a.simpleAudioVolume.SetMasterVolume(v, nil); err != nil {
		// AUDCLNT_E_DEVICE_INVALIDATED
		if err.(*ole.OleError).Code() == 0x88890004 {
			return fmt.Errorf("audio session %s unavailable", a.ProcessExecutable)
		}
		return fmt.Errorf("error setting volume: %w", err)
	}

	// Check if AudioSession is still active
	var s uint32
	if err := a.audioSessionControl2.GetState(&s); err != nil {
		return fmt.Errorf("error getting volume state: %w", err)
	}

	if s == wca.AudioSessionStateExpired {
		return errors.New("audio session state expired")
	}

	return nil
}

// New takes in a *wca.IAudioSessionControl and wraps it as an *AudioSession with some nice helper methods to do common tasks like SetVolumeLevel, etc.
func New(audioSessionEnumerator *wca.IAudioSessionEnumerator, audioSessionNumber int) (*AudioSession, error) {
	// This is an intermediate step of gathering the IAudioSessionControl so we can get IAudioSessionControl2 next.
	var audioSessionControl *wca.IAudioSessionControl
	if err := audioSessionEnumerator.GetSession(audioSessionNumber, &audioSessionControl); err != nil {
		return nil, fmt.Errorf("failed to get audio session control: %w", err)
	}
	defer audioSessionControl.Release()

	// This step is to pick up the IAudioSessionControl2 which enables the gathering of information about an audio session.
	dispatch, err := audioSessionControl.QueryInterface(wca.IID_IAudioSessionControl2)
	if err != nil {
		return nil, fmt.Errorf("failed to query interface: %w", err)
	}
	audioSessionControl2 := (*wca.IAudioSessionControl2)(unsafe.Pointer(dispatch))

	// Grabbing the processID is a quick trick to figure out if this audio session is the System Sounds audio session or not.
	var processId uint32
	if err := audioSessionControl2.GetProcessId(&processId); err != nil {
		// This error code 0x889000D just means it's a multiprocess and non-unique.
		// Which means it's the system sounds, if that's not the case then some error occured.
		if err.(*ole.OleError).Code() != 0x889000D {
			return nil, fmt.Errorf("failed to get process id: %w", err)
		}
	}

	// This next step gets us the ISimpleAudioVolume interface so that we can control the volume of this device.
	dispatch, err = audioSessionControl2.QueryInterface(wca.IID_ISimpleAudioVolume)
	if err != nil {
		return nil, fmt.Errorf("failed to get simple audio volume: %w", err)
	}
	simpleAudioVolume := (*wca.ISimpleAudioVolume)(unsafe.Pointer(dispatch))

	// Snagging the name of the process executible that created this audio session.
	var processExecutable string
	process, err := ps.FindProcess(int(processId))
	if err != nil {
		return nil, fmt.Errorf("failed to find process: %w", err)
	}
	processExecutable = process.Executable()

	as := &AudioSession{
		audioSessionControl2: audioSessionControl2,
		simpleAudioVolume:    simpleAudioVolume,
		ProcessExecutable:    processExecutable,
	}

	return as, nil
}
