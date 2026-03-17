package identity

import (
	"fmt"
	"os"
	"os/user"

	"github.com/rs/xid"
)

func WorkerID() string {
	return fmt.Sprintf("%s@%s/%s", username(), hostname(), xid.New().String())
}

func Origin() string {
	return fmt.Sprintf("%s@%s/%d", username(), hostname(), os.Getpid())
}

func username() string {
	u, err := user.Current()
	if err != nil {
		return "unknown"
	}
	return u.Username
}

func hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}
