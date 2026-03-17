package executor

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOsExecutor_SimpleEcho(t *testing.T) {
	e := NewOsExecutor()
	output, rc, err := e.Execute(context.Background(), "echo hello", "")
	require.NoError(t, err)
	assert.Equal(t, 0, rc)
	assert.Contains(t, output, "hello")
}

func TestOsExecutor_ArrayCommand(t *testing.T) {
	e := NewOsExecutor()
	output, rc, err := e.Execute(context.Background(), []interface{}{"echo", "world"}, "")
	require.NoError(t, err)
	assert.Equal(t, 0, rc)
	assert.Contains(t, output, "world")
}

func TestOsExecutor_ShellMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell test on unix")
	}
	e := NewOsExecutor()
	output, rc, err := e.Execute(context.Background(), "echo $((1+2))", "")
	require.NoError(t, err)
	assert.Equal(t, 0, rc)
	assert.Contains(t, output, "3")
}

func TestOsExecutor_NonZeroExit(t *testing.T) {
	e := NewOsExecutor()
	_, rc, err := e.Execute(context.Background(), "exit 42", "")
	require.NoError(t, err)
	assert.Equal(t, 42, rc)
}

func TestOsExecutor_CommandNotFound_Direct(t *testing.T) {
	e := NewOsExecutor()
	// Multi-element array bypasses shell, so os/exec returns an error
	_, _, err := e.Execute(context.Background(), []interface{}{"nonexistent_command_xyz", "arg"}, "")
	require.Error(t, err)
}

func TestOsExecutor_CommandNotFound_Shell(t *testing.T) {
	e := NewOsExecutor()
	// Single-element array goes through shell, returns non-zero rc
	_, rc, err := e.Execute(context.Background(), []interface{}{"nonexistent_command_xyz"}, "")
	require.NoError(t, err)
	assert.NotEqual(t, 0, rc)
}

func TestOsExecutor_Stderr(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("stderr test on unix")
	}
	e := NewOsExecutor()
	output, rc, err := e.Execute(context.Background(), "echo err >&2", "")
	require.NoError(t, err)
	assert.Equal(t, 0, rc)
	assert.Contains(t, output, "err")
}

func TestOsExecutor_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	e := NewOsExecutor()
	_, rc, err := e.Execute(ctx, "sleep 10", "")
	// Context cancel kills the shell; this may return error or non-zero rc
	if err == nil {
		assert.NotEqual(t, 0, rc)
	}
}

func TestOsExecutor_Chdir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chdir test on unix")
	}
	e := NewOsExecutor()
	output, rc, err := e.Execute(context.Background(), "pwd", "/tmp")
	require.NoError(t, err)
	assert.Equal(t, 0, rc)
	assert.Contains(t, output, "/tmp")
}

func TestOsExecutor_EmptyArray(t *testing.T) {
	e := NewOsExecutor()
	_, _, err := e.Execute(context.Background(), []interface{}{}, "")
	require.Error(t, err)
}

func TestOsExecutor_SingleElementArray(t *testing.T) {
	e := NewOsExecutor()
	output, rc, err := e.Execute(context.Background(), []interface{}{"echo hello"}, "")
	require.NoError(t, err)
	assert.Equal(t, 0, rc)
	assert.Contains(t, output, "hello")
}
