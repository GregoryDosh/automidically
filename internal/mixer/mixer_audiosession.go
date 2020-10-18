package mixer

import (
	ole "github.com/go-ole/go-ole"
	"github.com/moutend/go-wca/pkg/wca"
)

type AudioSession struct {
	ProcessExecutable    string
	audioSessionControl2 *wca.IAudioSessionControl2
	simpleAudioVolume    *wca.ISimpleAudioVolume
}

func (a *AudioSession) Cleanup() {
	mxaLog.Tracef("cleaning up %s", a.ProcessExecutable)
	if a.simpleAudioVolume != nil {
		a.simpleAudioVolume.Release()
	}
	if a.audioSessionControl2 != nil {
		a.audioSessionControl2.Release()
	}
}

func (a *AudioSession) SetVolume(v float32) bool {
	if (v < 0) || (1 < v) {
		mxaLog.Warnf("invalid volume level %f", v)
		return false
	}

	if err := a.simpleAudioVolume.SetMasterVolume(v, nil); err != nil {
		if err.(*ole.OleError).Code() == 0x88890004 { // AUDCLNT_E_DEVICE_INVALIDATED
			mxaLog.Debugf("audio session %s unavailable", a.ProcessExecutable)
			return false
		}
		mxaLog.Errorf("error setting volume: %v", err)
		return false
	}

	// Check if AudioSession is still active
	var s uint32
	if err := a.audioSessionControl2.GetState(&s); err != nil {
		mxaLog.Errorf("error getting volume state: %v", err)
		return false
	}

	if s == wca.AudioSessionStateExpired {
		mxaLog.Debug("audio session state expired")
		return false
	}

	return true
}
