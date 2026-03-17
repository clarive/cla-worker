package identity

import (
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
