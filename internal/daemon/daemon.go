package daemon

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func WritePID(pidfile string, pid int) error {
	return os.WriteFile(pidfile, []byte(fmt.Sprintf("%d\n", pid)), 0644)
}

func ReadPID(pidfile string) (int, error) {
	data, err := os.ReadFile(pidfile)
	if err != nil {
		return 0, fmt.Errorf("reading pidfile %s: %w", pidfile, err)
	}

	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return 0, fmt.Errorf("invalid pid in %s: %w", pidfile, err)
	}

	return pid, nil
}

func DeletePIDFile(pidfile string) error {
	if err := os.Remove(pidfile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("deleting pidfile %s: %w", pidfile, err)
	}
	return nil
}

func IsRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

type Status struct {
	Running bool
	PID     int
}

func GetStatus(pidfile string) Status {
	pid, err := ReadPID(pidfile)
	if err != nil {
		return Status{Running: false}
	}

	return Status{
		Running: IsRunning(pid),
		PID:     pid,
	}
}

func Kill(pidfile string) error {
	pid, err := ReadPID(pidfile)
	if err != nil {
		return err
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		DeletePIDFile(pidfile)
		return fmt.Errorf("process %d not found: %w", pid, err)
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		DeletePIDFile(pidfile)
		return fmt.Errorf("process %d cannot be killed: %w", pid, err)
	}

	for i := 0; i < 10; i++ {
		time.Sleep(1 * time.Second)
		if !IsRunning(pid) {
			DeletePIDFile(pidfile)
			return nil
		}
	}

	return fmt.Errorf("process %d did not stop within timeout", pid)
}
