package activewindow

import (
	"strings"
	"sync"

	"github.com/lxn/win"
	"github.com/mitchellh/go-ps"

	"github.com/sirupsen/logrus"
)

var (
	log                 = logrus.WithField("module", "activewindow")
	listenerLock        = &sync.Mutex{}
	activeWindowHandler win.HWINEVENTHOOK
	listener            *Listener
)

type Listener struct {
	processID       int
	processFilename string
	mutex           sync.Mutex
}

// newActiveWindowCallback is passed to Windows to be called whenever the active window changes.
// When it is called it will attempt to find the process of an associated handle, then get the executable associated with that.
func (l *Listener) newActiveWindowCallback(hWinEventHook win.HWINEVENTHOOK, event uint32, hwnd win.HWND, idObject int32, idChild int32, idEventThread uint32, dwmsEventTime uint32) (ret uintptr) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if hwnd == 0 {
		return
	}

	var pid uint32 = 0
	win.GetWindowThreadProcessId(hwnd, &pid)
	if pid == 0 {
		log.Debugf("unable to find PID from HWND %d", hwnd)
		return
	}
	l.processID = int(pid)

	p, err := ps.FindProcess(l.processID)
	if err != nil {
		log.Error(err)
		return
	}
	l.processFilename = strings.ToLower(p.Executable())

	log.WithFields(logrus.Fields{
		"filename": l.processFilename,
		"pid":      l.processID,
	}).Trace("new active window")

	return 0
}

// ProcessFilename is a safe way of getting the current active window's Process Filename
// This will always be converted to lowercase.
func (l *Listener) ProcessFilename() string {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	return l.processFilename
}

// ProcessID is a safe way of getting the current active window's Process ID
func (l *Listener) ProcessID() int {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	return l.processID
}

func GetListener() *Listener {
	listenerLock.Lock()
	defer listenerLock.Unlock()

	if listener == nil {
		listener = &Listener{}
		go startListenerMessageLoop()
	}

	return listener
}

func startListenerMessageLoop() {
	log.Trace("Starting Message Listener Loop")
	// The windows event hook will allow us to know when the active window has changed.
	handle, err := setActiveWindowWinEventHook(listener.newActiveWindowCallback)
	if err != nil {
		log.Fatal(err)
	}
	activeWindowHandler = handle

	msg := win.MSG{}
	for win.GetMessage(&msg, 0, 0, 0) != 0 {
		win.TranslateMessage(&msg)
		win.DispatchMessage(&msg)
	}
}

// setActiveWindowWinEventHook is for informing windows which function should be called whenever a
// foreground window has changed. https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-setwineventhook
func setActiveWindowWinEventHook(callbackFunction win.WINEVENTPROC) (win.HWINEVENTHOOK, error) {
	ret, err := win.SetWinEventHook(
		3,
		3,
		0,
		callbackFunction,
		0,
		0,
		win.WINEVENT_OUTOFCONTEXT|win.WINEVENT_SKIPOWNPROCESS,
	)
	if ret == 0 {
		return 0, err
	}

	return ret, nil
}
