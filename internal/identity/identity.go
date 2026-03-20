package identity

import (
	"fmt"
	"os"
	"os/user"

	"github.com/rs/xid"
)

func WorkerID() string {
	return fmt.Sprintf("%s@%s/%s", Username(), Hostname(), xid.New().String())
}

// DefaultWorkerName generates a worker name from user and server.
// Format: user@server-name (e.g. "joe@server1")
func DefaultWorkerName(user, server string) string {
	u := user
	if u == "" {
		u = Username()
	}
	s := server
	if s == "" {
		s = Hostname()
	}
	return fmt.Sprintf("%s@%s", u, s)
}

func Origin() string {
	return fmt.Sprintf("%s@%s/%d", Username(), Hostname(), os.Getpid())
}

// Username returns the current OS username.
func Username() string {
	return osUsername()
}

// Hostname returns the current OS hostname.
func Hostname() string {
	return osHostname()
}

func osUsername() string {
	u, err := user.Current()
	if err != nil {
		return "unknown"
	}
	return u.Username
}

func osHostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}
