package fsutils

import (
	"errors"
	"fmt"
	"os"
)

var stat = os.Stat

func FileExists(f string) (bool, error) {
	_, err := stat(f)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func DirExists(p string) (bool, error) {
	s, err := stat(p)
	if err == nil {
		if s.IsDir() {
			return true, nil
		}
		return false, fmt.Errorf("%s is not a directory", p)
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}
