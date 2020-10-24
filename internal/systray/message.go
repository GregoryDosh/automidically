package systray

type Message int

const (
	SystrayRefreshConfig Message = iota
	SystrayRefreshDevices
	SystrayRefreshSessions
	SystrayQuit
)
