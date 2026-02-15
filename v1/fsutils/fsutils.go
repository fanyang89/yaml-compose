package fsutils

import (
	"errors"
	"fmt"
	"os"
)

func FileExists(f string) (bool, error) {
	_, err := os.Stat(f)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func DirExists(p string) (bool, error) {
	s, err := os.Stat(p)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if s.IsDir() {
		return true, nil
	}
	return false, fmt.Errorf("%s is not a directory", p)
}
