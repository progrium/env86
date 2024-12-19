package fsutil

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"tractor.dev/toolkit-go/engine/fs"
)

func RootFSDir(path string) string {
	if runtime.GOOS == "windows" {
		return filepath.VolumeName(path)
	}
	return "/"
}

func RootFS(path string) fs.FS {
	return os.DirFS(RootFSDir(path))
}

/**
* RootFSRelativePath returns the path relative to the root of the filesystem.
* This is useful for checking if a file exists in a filesystem.
* On Unix:
* 	RootFSRelativePath("/foo/bar") => "foo/bar"
* On Windows:
* 	RootFSRelativePath("C:/foo/bar") => "foo/bar"
*/
func RootFSRelativePath(path string) string {
	if runtime.GOOS == "windows" {
		path = filepath.ToSlash(strings.TrimPrefix(path, filepath.VolumeName(path)))
	}
	return strings.TrimPrefix(path, "/")
}