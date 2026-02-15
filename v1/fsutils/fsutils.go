package fsutils

import (
	"errors"
	"fmt"

	"github.com/spf13/afero"
)

func FileExists(f string) (bool, error) {
	return FileExistsOn(afero.NewOsFs(), f)
}

func FileExistsOn(fs afero.Fs, f string) (bool, error) {
	_, err := fs.Stat(f)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, afero.ErrFileNotFound) {
		return false, nil
	}
	return false, err
}

func DirExists(p string) (bool, error) {
	return DirExistsOn(afero.NewOsFs(), p)
}

func DirExistsOn(fs afero.Fs, p string) (bool, error) {
	s, err := fs.Stat(p)
	if err == nil {
		if s.IsDir() {
			return true, nil
		}
		return false, fmt.Errorf("%s is not a directory", p)
	}
	if errors.Is(err, afero.ErrFileNotFound) {
		return false, nil
	}
	return false, err
}
