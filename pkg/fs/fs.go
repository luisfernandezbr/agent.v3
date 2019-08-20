package fs

import (
	"io"
	"os"
)

func WriteToTempAndRename(r io.Reader, loc string) error {
	temp := loc + ".tmp"
	f, err := os.Create(temp)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	if err != nil {
		return err
	}
	err = f.Close()
	if err != nil {
		return err
	}
	err = os.Rename(temp, loc)
	if err != nil {
		return err
	}
	return nil
}
