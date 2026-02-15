package fsutils

import (
	"errors"
	"fmt"
	"os"
)

type statFunc func(string) (os.FileInfo, error)

func FileExists(f string) (bool, error) {
	return fileExistsWithStat(os.Stat, f)
}

func fileExistsWithStat(stat statFunc, f string) (bool, error) {
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
	return dirExistsWithStat(os.Stat, p)
}

func dirExistsWithStat(stat statFunc, p string) (bool, error) {
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
