package archive

import (
	"archive/zip"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

func Unzip(target string, archive string) error {
	r, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer func() {
		err := r.Close()
		if err != nil {
			panic(err)
		}
	}()

	err = os.MkdirAll(target, 0777)
	if err != nil {
		return err
	}

	for _, f := range r.File {
		err := unzipFile(f, target)
		if err != nil {
			return err
		}
	}

	return nil
}

func unzipFile(zf *zip.File, target string) error {

	r, err := zf.Open()
	if err != nil {
		return err
	}

	defer func() {
		err := r.Close()
		if err != nil {
			panic(err)
		}
	}()

	fpath := filepath.Join(target, zf.Name)

	if zf.FileInfo().IsDir() {
		err := os.MkdirAll(fpath, 0777)
		if err != nil {
			return err
		}
		return nil
	}

	err = os.MkdirAll(filepath.Dir(fpath), 0777)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}
	defer func() {
		err := f.Close()
		if err != nil {
			panic(err)
		}
	}()
	_, err = io.Copy(f, r)
	if err != nil {
		return err
	}
	return nil
}

// ZipFiles compresses one or many files into a single zip archive file
func ZipFiles(filename, baseDir string, files []string) error {
	newfile, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer newfile.Close()

	zipWriter := zip.NewWriter(newfile)
	defer zipWriter.Close()

	// Add files to zip
	for _, file := range files {
		err := copyFile(file, baseDir, zipWriter)

		if err != nil {
			return err
		}
	}
	return nil
}

func ZipDir(target string, dir string) error {
	newfile, err := os.Create(target)
	if err != nil {
		return err
	}
	defer func() {
		err := newfile.Close()
		if err != nil {
			panic(err)
		}
	}()

	zipWriter := zip.NewWriter(newfile)
	defer func() {
		err := zipWriter.Close()
		if err != nil {
			panic(err)
		}
	}()

	baseDir := dir

	var rec func(string) error
	rec = func(dir string) error {
		items, err := ioutil.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, item := range items {
			p := filepath.Join(dir, item.Name())
			if item.IsDir() {
				err := createFolder(p, baseDir, zipWriter)
				if err != nil {
					return err
				}
				err = rec(p)
				if err != nil {
					return err
				}
				continue
			}
			err := copyFile(p, baseDir, zipWriter)
			if err != nil {
				return err
			}
		}
		return nil
	}
	return rec(dir)
}

func createFolder(loc, baseDir string, zipWriter *zip.Writer) error {
	newPath, _ := filepath.Rel(baseDir, loc)
	newPath = filepath.ToSlash(newPath) + "/"

	_, err := zipWriter.Create(newPath)
	if err != nil {
		return err
	}

	return nil
}

func copyFile(file, baseDir string, zipWriter *zip.Writer) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer func() {
		err := f.Close()
		if err != nil {
			panic(err)
		}
	}()

	// Get the file information
	info, err := f.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}

	// Change to deflate to gain better compression
	// see http://golang.org/pkg/archive/zip/#pkg-constants
	header.Method = zip.Deflate
	newPath, _ := filepath.Rel(baseDir, file)
	header.Name = filepath.ToSlash(newPath)

	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(writer, f)
	if err != nil {
		return err
	}

	return nil
}
