module github.com/GregoryDosh/automidically

go 1.15

require (
	github.com/bep/debounce v1.2.0
	github.com/fsnotify/fsnotify v1.4.9
	github.com/getlantern/systray v1.0.5
	github.com/go-ole/go-ole v1.2.4
	github.com/go-toast/toast v0.0.0-20190211030409-01e6764cf0a4
	github.com/mitchellh/go-ps v1.0.0
	github.com/moutend/go-wca v0.2.0
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d // indirect
	github.com/orandin/lumberjackrus v1.0.1
	github.com/sirupsen/logrus v1.7.0
	github.com/stretchr/testify v1.6.1 // indirect
	github.com/urfave/cli/v2 v2.2.0
	gitlab.com/gomidi/midi v1.20.1
	gitlab.com/gomidi/rtmididrv v0.10.1
	golang.org/x/sys v0.0.0-20201018230417-eeed37f84f13 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776
)

replace github.com/moutend/go-wca => github.com/GregoryDosh/go-wca v0.2.1-0.20201024160608-e13d0c92135e
