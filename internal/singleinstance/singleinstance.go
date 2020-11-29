package singleinstance

import (
	"errors"
	"syscall"

	"golang.org/x/sys/windows"
)

var InstanceAlreadyExistsError = errors.New("unable to get instance lock")

func GetLock() error {
	// https://docs.microsoft.com/en-us/windows/win32/api/synchapi/nf-synchapi-createmutexexw
	name, err := syscall.UTF16PtrFromString("Local\\b3d17eec-fb55-43ad-9a6e-f44946165bd1")
	if err != nil {
		return err
	}

	var flags uint32 = 0x00000001
	var desiredAccess uint32 = 0x00000000
	if _, err := windows.CreateMutexEx(nil, name, flags, desiredAccess); err != nil {
		return InstanceAlreadyExistsError
	}
	return nil
}
