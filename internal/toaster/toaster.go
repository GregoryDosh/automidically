package toaster

import (
	"encoding/json"
	"fmt"

	"github.com/go-toast/toast"
	"github.com/sirupsen/logrus"
)

type Toast struct {
	minLevel  logrus.Level
	formatter logrus.Formatter
}

func (t *Toast) Fire(entry *logrus.Entry) error {
	msg, err := t.formatter.Format(entry)
	if err != nil {
		return err
	}

	f := map[string]interface{}{}
	if err := json.Unmarshal(msg, &f); err != nil {
		return err
	}

	l, ok := f["level"]
	if !ok {
		l = "Unknown"
	}

	m, ok := f["msg"]
	if !ok {
		m = "Unknown message."
	}

	notification := toast.Notification{
		AppID:   "AutoMIDIcally",
		Title:   fmt.Sprintf("AutoMIDIcally - %s", l),
		Message: m.(string),
	}
	err = notification.Push()
	if err != nil {
		return err
	}

	return nil
}

func (t *Toast) Levels() []logrus.Level {
	return logrus.AllLevels[:t.minLevel+1]
}

func New(level logrus.Level, formatter logrus.Formatter) *Toast {
	return &Toast{
		minLevel:  level,
		formatter: formatter,
	}
}
