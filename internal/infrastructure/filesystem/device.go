package filesystem

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

var errNoDeviceNumber = errors.New("the filesystem reports no device number")

func SameFilesystem(first, second string) (bool, error) {
	firstDevice, err := deviceOf(first)
	if err != nil {
		return false, err
	}
	secondDevice, err := deviceOf(second)
	if err != nil {
		return false, err
	}
	return firstDevice == secondDevice, nil
}

func deviceOf(path string) (uint64, error) {
	details, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	status, isUnixStatus := details.Sys().(*syscall.Stat_t)
	if !isUnixStatus {
		return 0, fmt.Errorf("%w: %s", errNoDeviceNumber, path)
	}
	return status.Dev, nil
}
