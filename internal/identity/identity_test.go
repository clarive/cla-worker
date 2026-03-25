package identity

import (
	"net"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWorkerID_Format(t *testing.T) {
	id := WorkerID()
	assert.Contains(t, id, "@")
	assert.Contains(t, id, "/")
	parts := strings.SplitN(id, "/", 2)
	assert.Len(t, parts, 2)
	assert.NotEmpty(t, parts[1])
}

func TestWorkerID_UniquePerCall(t *testing.T) {
	id1 := WorkerID()
	id2 := WorkerID()
	assert.NotEqual(t, id1, id2)
}

func TestOrigin_Format(t *testing.T) {
	o := Origin()
	re := regexp.MustCompile(`^.+@.+/\d+$`)
	assert.Regexp(t, re, o)
}

func TestOrigin_ContainsPID(t *testing.T) {
	o := Origin()
	parts := strings.Split(o, "/")
	pidStr := parts[len(parts)-1]
	assert.NotEmpty(t, pidStr)
}

func TestUsername_ReturnsNonEmpty(t *testing.T) {
	u := Username()
	assert.NotEmpty(t, u)
	assert.NotEqual(t, "unknown", u)
}

func TestHostname_ReturnsNonEmpty(t *testing.T) {
	h := Hostname()
	assert.NotEmpty(t, h)
	assert.NotEqual(t, "unknown", h)
}

func TestDefaultWorkerName_UsesOSDefaults(t *testing.T) {
	name := DefaultWorkerName("", "")
	assert.Contains(t, name, "@")
	assert.Equal(t, Username()+"@"+Hostname(), name)
}

func TestDefaultWorkerName_OverridesUser(t *testing.T) {
	name := DefaultWorkerName("alice", "")
	assert.Equal(t, "alice@"+Hostname(), name)
}

func TestDefaultWorkerName_OverridesServer(t *testing.T) {
	name := DefaultWorkerName("", "mybox")
	assert.Equal(t, Username()+"@mybox", name)
}

func TestIsIPAddress(t *testing.T) {
	assert.True(t, isIPAddress("192.168.1.139"))
	assert.True(t, isIPAddress("10.0.0.1"))
	assert.True(t, isIPAddress("::1"))
	assert.True(t, isIPAddress("fe80::1"))
	assert.False(t, isIPAddress("myhost"))
	assert.False(t, isIPAddress("server1.local"))
	assert.False(t, isIPAddress("foo-bar"))
	assert.False(t, isIPAddress(""))
}

func TestIsValidHostname(t *testing.T) {
	// claude: valid hostnames
	assert.True(t, isValidHostname("foo"))
	assert.True(t, isValidHostname("server1"))
	assert.True(t, isValidHostname("bar-baz.local"))

	// claude: reject IPs, spaces, empty
	assert.False(t, isValidHostname("10.0.0.1"))
	assert.False(t, isValidHostname("Some Laptop"))
	assert.False(t, isValidHostname("My Computer"))
	assert.False(t, isValidHostname(""))
}

func TestOsUsername_StripsDomain(t *testing.T) {
	// claude: on Windows, user.Current().Username returns "DOMAIN\user";
	// verify our stripping logic works by checking the result has no backslash
	u := osUsername()
	assert.NotContains(t, u, `\`, "username should not contain backslash domain prefix, got %q", u)
}

func TestHostname_PrefersNameOverIP(t *testing.T) {
	h := Hostname()
	// claude: hostname should not be an IP address unless no other option exists
	if net.ParseIP(h) != nil {
		t.Logf("WARNING: Hostname() returned IP address %q — no real hostname available on this machine", h)
	} else {
		assert.NotEmpty(t, h)
		assert.NotEqual(t, "unknown", h)
		assert.False(t, strings.Contains(h, " "), "hostname should not contain spaces, got %q", h)
	}
}
