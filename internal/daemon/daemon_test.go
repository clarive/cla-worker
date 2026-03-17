package daemon

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWritePID(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.pid")

	err := WritePID(f, 12345)
	require.NoError(t, err)

	data, err := os.ReadFile(f)
	require.NoError(t, err)
	assert.Contains(t, string(data), "12345")
}

func TestReadPID_Valid(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.pid")
	os.WriteFile(f, []byte("54321\n"), 0644)

	pid, err := ReadPID(f)
	require.NoError(t, err)
	assert.Equal(t, 54321, pid)
}

func TestReadPID_Empty(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.pid")
	os.WriteFile(f, []byte(""), 0644)

	_, err := ReadPID(f)
	require.Error(t, err)
}

func TestReadPID_Invalid(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.pid")
	os.WriteFile(f, []byte("not-a-number\n"), 0644)

	_, err := ReadPID(f)
	require.Error(t, err)
}

func TestReadPID_NotFound(t *testing.T) {
	_, err := ReadPID("/nonexistent/pid")
	require.Error(t, err)
}

func TestDeletePIDFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.pid")
	os.WriteFile(f, []byte("123\n"), 0644)

	err := DeletePIDFile(f)
	require.NoError(t, err)

	_, err = os.Stat(f)
	assert.True(t, os.IsNotExist(err))
}

func TestDeletePIDFile_NotExists(t *testing.T) {
	err := DeletePIDFile("/nonexistent/pid")
	require.NoError(t, err)
}

func TestIsRunning_CurrentProcess(t *testing.T) {
	assert.True(t, IsRunning(os.Getpid()))
}

func TestIsRunning_InvalidPID(t *testing.T) {
	assert.False(t, IsRunning(99999999))
}

func TestGetStatus_NoFile(t *testing.T) {
	status := GetStatus("/nonexistent/pid")
	assert.False(t, status.Running)
}

func TestGetStatus_Running(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.pid")
	WritePID(f, os.Getpid())

	status := GetStatus(f)
	assert.True(t, status.Running)
	assert.Equal(t, os.Getpid(), status.PID)
}

func TestGetStatus_NotRunning(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.pid")
	os.WriteFile(f, []byte("99999999\n"), 0644)

	status := GetStatus(f)
	assert.False(t, status.Running)
}
