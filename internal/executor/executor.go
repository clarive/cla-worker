package executor

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
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
		command = exec.CommandContext(ctx, shell, flag, sanitizeCmdWindows(c))
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
			command = exec.CommandContext(ctx, shell, flag, sanitizeCmdWindows(args[0]))
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

// claude: sanitizeCmdWindows works around a cmd.exe /C parsing quirk where
// a trailing backslash is treated as an escape character for the next
// character in its internal quote/argument parsing. Appending a space
// after the trailing backslash prevents cmd.exe from eating the closing
// quote or delimiter. Example: "dir c:\" fails, "dir c:\ " works.
func sanitizeCmdWindows(cmd string) string {
	if runtime.GOOS == "windows" && strings.HasSuffix(cmd, `\`) {
		return cmd + " "
	}
	return cmd
}
