package executor

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
)

type CommandExecutor interface {
	Execute(ctx context.Context, cmd interface{}, chdir string) (output string, rc int, err error)
}

type OsExecutor struct{}

func NewOsExecutor() *OsExecutor {
	return &OsExecutor{}
}

func (e *OsExecutor) Execute(ctx context.Context, cmd interface{}, chdir string) (string, int, error) {
	var command *exec.Cmd

	switch c := cmd.(type) {
	case string:
		shell, flag := shellCommand()
		command = exec.CommandContext(ctx, shell, flag, c)
	case []interface{}:
		if len(c) == 0 {
			return "", 1, fmt.Errorf("empty command array")
		}
		args := make([]string, len(c))
		for i, a := range c {
			args[i] = fmt.Sprintf("%v", a)
		}
		if len(args) == 1 {
			shell, flag := shellCommand()
			command = exec.CommandContext(ctx, shell, flag, args[0])
		} else {
			command = exec.CommandContext(ctx, args[0], args[1:]...)
		}
	default:
		return "", 1, fmt.Errorf("unsupported command type: %T", cmd)
	}

	if chdir != "" {
		command.Dir = chdir
	}

	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output

	err := command.Run()
	rc := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			rc = exitErr.ExitCode()
		} else {
			return output.String(), 1, err
		}
	}

	return output.String(), rc, nil
}

func shellCommand() (string, string) {
	if runtime.GOOS == "windows" {
		return "cmd", "/C"
	}
	return "/bin/sh", "-c"
}
