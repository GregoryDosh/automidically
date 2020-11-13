package activewindow

import (
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"github.com/mitchellh/go-ps"
	"github.com/sirupsen/logrus"
)

var (
	log                      = logrus.WithField("module", "activewindow")
	user32                   = syscall.MustLoadDLL("user32.dll")
	setWinEventHook          = user32.MustFindProc("SetWinEventHook")
	unhookWinEvent           = user32.MustFindProc("UnhookWinEvent")
	getWindowThreadProcessId = user32.MustFindProc("GetWindowThreadProcessId")
	handlerLock              = &sync.Mutex{}
	processID                int
	processFilename          string
	activeWindowHandler      uintptr
)

// init will create an active window change handler and since we really don't
// need multiple instances watching and listening for on change events it will only create one.
func init() {
	// The windows event hook will allow us to know when the active window has changed.
	handle, err := setActiveWindowWinEventHook(newActiveWindowCallback)
	if err != nil {
		log.Fatal(err)
		return
	}
	activeWindowHandler = handle
}

// Cleanup will unreigster the windows handler event.
func Cleanup() {
	handlerLock.Lock()
	defer handlerLock.Unlock()
	if activeWindowHandler > 0 {
		log.Trace("cleaning up active window handler")
		unhookActiveWindowWinEventHook(activeWindowHandler)
	}
	processID = 0
	processFilename = ""
}

// ProcessFilename is a safe way of getting the current active window's Process Filename
// This will always be converted to lowercase.
func ProcessFilename() string {
	handlerLock.Lock()
	defer handlerLock.Unlock()
	return processFilename
}

// ProcessID is a safe way of getting the current active window's Process ID
func ProcessID() int {
	handlerLock.Lock()
	defer handlerLock.Unlock()
	return processID
}

// getPIDFromHWND will get the PID of a particular HWND
func getPIDFromHWND(hwnd uintptr) uintptr {
	var prcsId uintptr = 0
	ret, _, err := getWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&prcsId)))
	if ret == 0 {
		log.Error(err)
		return 0
	}
	return prcsId
}

// newActiveWindowCallback is passed to Windows to be called whenever the active window changes.
// When it is called it will attempt to find the process of an associated handle, then get the executable associated with that.
func newActiveWindowCallback(hWinEventHook uintptr, event uint32, hwnd uintptr, idObject int32, idChild int32, idEventThread uint32, dwmsEventTime uint32) (ret uintptr) {
	handlerLock.Lock()
	defer handlerLock.Unlock()
	if hwnd == 0 {
		return
	}
	pid := getPIDFromHWND(hwnd)
	if pid == 0 {
		log.Debugf("unable to find PID from HWND %d", hwnd)
		return
	}
	processID = int(pid)

	p, err := ps.FindProcess(processID)
	if err != nil {
		log.Error(err)
		return
	}
	processFilename = strings.ToLower(p.Executable())

	log.Tracef("new active window '%s' (%d)", processFilename, processID)
	return
}

// setActiveWindowWinEventHook is for informing windows which function should be called whenever a
// foreground window has changed. https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-setwineventhook
func setActiveWindowWinEventHook(callbackFunction func(hWinEventHook uintptr, event uint32, hwnd uintptr, idObject int32, idChild int32, idEventThread uint32, dwmsEventTime uint32) uintptr) (uintptr, error) {
	ret, _, err := syscall.Syscall9(setWinEventHook.Addr(), 7, 3, 3, 0, syscall.NewCallback(callbackFunction), 0, 0, 3, 0, 0)
	if ret == 0 {
		return 0, err
	}
	return ret, nil
}

// unhookActiveWindowWinEventHook will release the hook set by the setActiveWindowWinEventHook above using the returned value
func unhookActiveWindowWinEventHook(hWinHookEvent uintptr) bool {
	ret, _, _ := syscall.Syscall(unhookWinEvent.Addr(), 1, hWinHookEvent, 0, 0)
	return ret != 0
}
