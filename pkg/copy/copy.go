package copy

import (
	"io"
	"os"
	"io/ioutil"
	"path"
	"fmt"
	"github.com/palantir/stacktrace"
)

// File copies a single file from src to dst
func File(src, dst string) error {
	var err error
	var srcfd *os.File
	var dstfd *os.File
	var srcinfo os.FileInfo

	if srcfd, err = os.Open(src); err != nil {
		return stacktrace.Propagate(err, "")
	}
	defer srcfd.Close()

	if dstfd, err = os.Create(dst); err != nil {
		return stacktrace.Propagate(err, "")
	}
	defer dstfd.Close()

	if _, err = io.Copy(dstfd, srcfd); err != nil {
		return stacktrace.Propagate(err, "")
	}
	if srcinfo, err = os.Stat(src); err != nil {
		return stacktrace.Propagate(err, "")
	}
	return os.Chmod(dst, srcinfo.Mode())
}


// Dir copies a whole directory recursively
func Dir(src string, dst string) error {
	var err error
	var fds []os.FileInfo
	var srcinfo os.FileInfo

	if srcinfo, err = os.Stat(src); err != nil {
		return stacktrace.Propagate(err, "")
	}

	if err = os.MkdirAll(dst, srcinfo.Mode()); err != nil {
		return stacktrace.Propagate(err, "")
	}

	if fds, err = ioutil.ReadDir(src); err != nil {
		return stacktrace.Propagate(err, "")
	}
	for _, fd := range fds {
		srcfp := path.Join(src, fd.Name())
		dstfp := path.Join(dst, fd.Name())

		if fd.IsDir() {
			if err = Dir(srcfp, dstfp); err != nil {
				fmt.Println(err)
			}
		} else {
			if err = File(srcfp, dstfp); err != nil {
				fmt.Println(err)
			}
		}
	}
	return nil
}