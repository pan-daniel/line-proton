package filesystem

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

type Lock struct {
	path string
}

func NewLock(path string) Lock {
	return Lock{path: path}
}

func nothingToRelease() error {
	return nil
}

func (lock Lock) Acquire() (func() error, error) {
	return lock.lockWith(syscall.LOCK_EX)
}

func (lock Lock) TryAcquire() (func() error, bool, error) {
	release, err := lock.lockWith(syscall.LOCK_EX | syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return nothingToRelease, false, nil
	}
	return release, err == nil, err
}

func (lock Lock) lockWith(operation int) (func() error, error) {
	if err := os.MkdirAll(filepath.Dir(lock.path), 0o700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(lock.path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), operation); err != nil {
		return nil, errors.Join(err, file.Close())
	}
	return releaseOf(file), nil
}

func releaseOf(file *os.File) func() error {
	return func() error {
		return errors.Join(syscall.Flock(int(file.Fd()), syscall.LOCK_UN), file.Close())
	}
}
