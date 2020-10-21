package message

type Message int

const (
	SystrayRefreshConfig Message = iota
	SystrayRefreshDevices
	SystrayRefreshSessions
)
