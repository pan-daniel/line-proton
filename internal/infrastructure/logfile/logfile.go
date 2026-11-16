package logfile

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

func Open(path string, limit int64) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	if err := rotateWhenOver(path, limit); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
}

func rotateWhenOver(path string, limit int64) error {
	details, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if details.Size() <= limit {
		return nil
	}
	return os.Rename(path, path+".1")
}
