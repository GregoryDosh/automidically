package shell

import (
	"os/exec"
	"syscall"

	sysmsg "github.com/GregoryDosh/automidically/internal/systray"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField("module", "shell")

type Mapping struct {
	Cc             int      `yaml:"cc"`
	Command        []string `yaml:"-"`
	LogOutput      bool     `yaml:"log_output"`
	SuppressErrors bool     `yaml:"suppress_errors"`
	UsePowershell  bool     `yaml:"use_powershell"`
}

func (m *Mapping) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// This is so we can set some default values if not specified in the config.
	type rawMapping Mapping
	var raw rawMapping
	if err := unmarshal(&raw); err != nil {
		return err
	}
	*m = Mapping(raw)

	// Letting command take on a string or string slice
	// requires a little bit of massaging to get into the struct.
	cString := struct{ Command string }{}
	if err := unmarshal(&cString); err == nil {
		m.Command = []string{cString.Command}
	}

	cSlice := struct{ Command []string }{}
	if err := unmarshal(&cSlice); err == nil {
		m.Command = cSlice.Command
	}

	return nil
}

func (m *Mapping) HandleMIDIMessage(c int, v int) {
	if m.Cc != c {
		return
	}

	exe := "cmd.exe"
	args := []string{"/C"}
	if m.UsePowershell {
		exe = "powershell.exe"
		args = []string{"-NoProfile", "-NonInteractive"}
	}

	args = append(args, m.Command...)

	cmd := exec.Command(exe, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	output, err := cmd.CombinedOutput()
	if err != nil && !m.SuppressErrors {
		log.Errorf("%s returned error %s", m.Command, err)
		return
	}
	if m.LogOutput && len(output) > 0 {
		log.Infof("%s returned %s", m.Command, output)
	}
}

func HandleSystrayMessage(msg sysmsg.Message) {
}
