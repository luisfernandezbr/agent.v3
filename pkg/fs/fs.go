package fs

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

func ChmodFilesInDir(dir string, mode os.FileMode) error {
	items, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, item := range items {
		n := filepath.Join(dir, item.Name())
		if item.IsDir() {
			err := ChmodFilesInDir(n, mode)
			if err != nil {
				return err
			}
			continue
		}
		err := os.Chmod(n, mode)
		if err != nil {
			return err
		}
	}
	return nil
}

func Copy(src, dst string) error {
	fi, err := os.Stat(src)
	if err != nil {
		return err
	}
	if fi.IsDir() {
		return CopyDir(src, dst)
	}
	return CopyFile(src, dst)
}

func CopyDir(src, dst string) error {
	err := os.MkdirAll(dst, 0755)
	if err != nil {
		return err
	}
	items, err := ioutil.ReadDir(src)
	if err != nil {
		return err
	}
	for _, item := range items {
		s := filepath.Join(src, item.Name())
		d := filepath.Join(dst, item.Name())

		if item.IsDir() {
			err := CopyDir(s, d)
			if err != nil {
				return err
			}
			continue
		}
		err := CopyFile(s, d)
		if err != nil {
			return err
		}
	}
	return nil
}

func CopyFile(src, dst string) error {

	srcf, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcf.Close()

	err = os.MkdirAll(filepath.Dir(dst), 0755)
	if err != nil {
		return err
	}
	dstf, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstf.Close()
	_, err = io.Copy(dstf, srcf)
	if err != nil {
		return err
	}
	err = dstf.Sync()
	if err != nil {
		return err
	}
	err = dstf.Close()
	if err != nil {
		return err
	}
	return nil
}

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

func Exists(loc string) (bool, error) {
	_, err := os.Stat(loc)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
