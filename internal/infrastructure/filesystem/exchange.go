package filesystem

import (
	"os"

	"golang.org/x/sys/unix"
)

func Exchange(first, second string) error {
	if err := unix.Renameat2(unix.AT_FDCWD, first, unix.AT_FDCWD, second, unix.RENAME_EXCHANGE); err != nil {
		return &os.LinkError{Op: "exchange", Old: first, New: second, Err: err}
	}
	return nil
}
