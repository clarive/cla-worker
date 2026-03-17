package filetransfer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteFileAtomic_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "output.txt")

	fs := NewOsFileSystem()
	err := fs.WriteFileAtomic(path, strings.NewReader("hello world"))
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "hello world", string(data))
}

func TestWriteFileAtomic_CreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "dir", "output.txt")

	fs := NewOsFileSystem()
	err := fs.WriteFileAtomic(path, strings.NewReader("nested"))
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "nested", string(data))
}

func TestWriteFileAtomic_CleanupOnError(t *testing.T) {
	fs := NewOsFileSystem()
	err := fs.WriteFileAtomic("/nonexistent/path/that/fails/file.txt",
		strings.NewReader("data"))
	require.Error(t, err)
}

func TestReadFile_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "read.txt")
	err := os.WriteFile(path, []byte("read me"), 0644)
	require.NoError(t, err)

	fs := NewOsFileSystem()
	f, err := fs.ReadFile(path)
	require.NoError(t, err)
	defer f.Close()

	buf := make([]byte, 100)
	n, _ := f.Read(buf)
	assert.Equal(t, "read me", string(buf[:n]))
}

func TestReadFile_NotFound(t *testing.T) {
	fs := NewOsFileSystem()
	_, err := fs.ReadFile("/nonexistent/file.txt")
	require.Error(t, err)
}

func TestExists_File(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "exists.txt")
	os.WriteFile(path, []byte("x"), 0644)

	fs := NewOsFileSystem()
	exists, isDir, err := fs.Exists(path)
	require.NoError(t, err)
	assert.True(t, exists)
	assert.False(t, isDir)
}

func TestExists_Directory(t *testing.T) {
	dir := t.TempDir()

	fs := NewOsFileSystem()
	exists, isDir, err := fs.Exists(dir)
	require.NoError(t, err)
	assert.True(t, exists)
	assert.True(t, isDir)
}

func TestExists_Missing(t *testing.T) {
	fs := NewOsFileSystem()
	exists, _, err := fs.Exists("/nonexistent/path/xyz")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestWriteFileAtomic_Overwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "overwrite.txt")
	os.WriteFile(path, []byte("old"), 0644)

	fs := NewOsFileSystem()
	err := fs.WriteFileAtomic(path, strings.NewReader("new"))
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "new", string(data))
}
