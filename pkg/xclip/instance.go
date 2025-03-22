package xclip

import (
	"os/exec"
)

type XClip struct {
	RunFn func(*exec.Cmd) error
}

var Cli = &XClip{
	RunFn: realRunner,
}

func realRunner(cmd *exec.Cmd) error {
	return cmd.Run()
}
