package filetransfer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type FileSystem interface {
	WriteFileAtomic(filepath string, r io.Reader) error
	ReadFile(filepath string) (*os.File, error)
	Exists(path string) (bool, bool, error)
}

type OsFileSystem struct{}

func NewOsFileSystem() *OsFileSystem {
	return &OsFileSystem{}
}

func (fs *OsFileSystem) WriteFileAtomic(path string, r io.Reader) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".cla-tmp-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()

	defer func() {
		if tmpPath != "" {
			os.Remove(tmpPath)
		}
	}()

	if _, err := io.Copy(tmp, r); err != nil {
		tmp.Close()
		return fmt.Errorf("writing temp file: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("renaming temp to final: %w", err)
	}

	tmpPath = ""
	return nil
}

func (fs *OsFileSystem) ReadFile(path string) (*os.File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening file %s: %w", path, err)
	}
	return f, nil
}

func (fs *OsFileSystem) Exists(path string) (exists bool, isDir bool, err error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, false, nil
		}
		return false, false, err
	}
	return true, info.IsDir(), nil
}
