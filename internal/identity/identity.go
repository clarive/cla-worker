package identity

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/user"
	"strings"

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

// claude: isIPAddress checks if a string looks like an IPv4 or IPv6 address.
func isIPAddress(s string) bool {
	return net.ParseIP(s) != nil
}

// claude: isValidHostname checks that a string is usable as a worker hostname:
// not empty, not an IP, and no spaces (rejects display names like "MacBook Pro").
func isValidHostname(s string) bool {
	return s != "" && !isIPAddress(s) && !strings.ContainsRune(s, ' ')
}

// claude: osHostname tries multiple sources to get a real hostname,
// using IP address only as the absolute last resort.
func osHostname() string {
	// 1. Try os.Hostname() — best case, returns a real name
	if h, err := os.Hostname(); err == nil && isValidHostname(h) {
		return h
	}

	// 2. Try scutil --get HostName (macOS explicitly set hostname)
	if out, err := exec.Command("scutil", "--get", "HostName").Output(); err == nil {
		if name := strings.TrimSpace(string(out)); isValidHostname(name) {
			return name
		}
	}

	// 3. Try the hostname command (returns FQDN like "m4c.local")
	if out, err := exec.Command("hostname").Output(); err == nil {
		if name := strings.TrimSpace(string(out)); isValidHostname(name) {
			return name
		}
	}

	// 4. Try the HOSTNAME env var
	if h := os.Getenv("HOSTNAME"); isValidHostname(h) {
		return h
	}

	// 5. Try scutil --get LocalHostName (macOS Bonjour name, e.g. "m4c")
	if out, err := exec.Command("scutil", "--get", "LocalHostName").Output(); err == nil {
		if name := strings.TrimSpace(string(out)); isValidHostname(name) {
			return name
		}
	}

	// 6. Last resort: return whatever os.Hostname gives, even if it's an IP
	if h, err := os.Hostname(); err == nil && h != "" {
		return h
	}

	return "unknown"
}
