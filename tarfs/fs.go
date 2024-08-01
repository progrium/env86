// package tarfs implements a read-only in-memory representation of a tar archive
package tarfs

import (
	"archive/tar"
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

var Separator = string(filepath.Separator)

type FS struct {
	files map[string]map[string]*File
}

func splitpath(name string) (dir, file string) {
	name = filepath.ToSlash(name)
	if len(name) == 0 || name[0] != '/' {
		name = "/" + name
	}
	name = filepath.Clean(name)
	dir, file = filepath.Split(name)
	dir = filepath.Clean(dir)
	return
}

func New(t *tar.Reader) *FS {
	fs := &FS{files: make(map[string]map[string]*File)}
	for {
		hdr, err := t.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil
		}

		d, f := splitpath(hdr.Name)
		if _, ok := fs.files[d]; !ok {
			fs.files[d] = make(map[string]*File)
		}

		var buf bytes.Buffer
		size, err := buf.ReadFrom(t)
		if err != nil {
			panic("tarfs: reading from tar:" + err.Error())
		}

		if size != hdr.Size {
			panic("tarfs: size mismatch")
		}

		file := &File{
			h:    hdr,
			data: bytes.NewReader(buf.Bytes()),
			fs:   fs,
		}
		fs.files[d][f] = file

	}

	if fs.files[Separator] == nil {
		fs.files[Separator] = make(map[string]*File)
	}
	// Add a pseudoroot
	fs.files[Separator][""] = &File{
		h: &tar.Header{
			Name:     Separator,
			Typeflag: tar.TypeDir,
			Size:     0,
		},
		data: bytes.NewReader(nil),
		fs:   fs,
	}

	return fs
}

func (fs *FS) Open(name string) (fs.File, error) {
	d, f := splitpath(name)
	if _, ok := fs.files[d]; !ok {
		return nil, &os.PathError{Op: "open", Path: name, Err: syscall.ENOENT}
	}

	file, ok := fs.files[d][f]
	if !ok {
		return nil, &os.PathError{Op: "open", Path: name, Err: syscall.ENOENT}
	}

	nf := *file

	return &nf, nil
}

func (fs *FS) Name() string { return "tarfs" }

func (fs *FS) Create(name string) (fs.File, error) { return nil, syscall.EROFS }

func (fs *FS) Mkdir(name string, perm os.FileMode) error { return syscall.EROFS }

func (fs *FS) MkdirAll(path string, perm os.FileMode) error { return syscall.EROFS }

func (fs *FS) OpenFile(name string, flag int, perm os.FileMode) (fs.File, error) {
	if flag != os.O_RDONLY {
		return nil, &os.PathError{Op: "open", Path: name, Err: syscall.EPERM}
	}

	return fs.Open(name)
}

func (fs *FS) Remove(name string) error { return syscall.EROFS }

func (fs *FS) RemoveAll(path string) error { return syscall.EROFS }

func (fs *FS) Rename(oldname string, newname string) error { return syscall.EROFS }

func (fs *FS) Stat(name string) (fs.FileInfo, error) {
	d, f := splitpath(name)
	if _, ok := fs.files[d]; !ok {
		return nil, &os.PathError{Op: "stat", Path: name, Err: syscall.ENOENT}
	}

	file, ok := fs.files[d][f]
	if !ok {
		return nil, &os.PathError{Op: "stat", Path: name, Err: syscall.ENOENT}
	}

	return file.h.FileInfo(), nil
}

func (fs *FS) Chmod(name string, mode fs.FileMode) error { return syscall.EROFS }

func (fs *FS) Chown(name string, uid, gid int) error { return syscall.EROFS }

func (fs *FS) Chtimes(name string, atime time.Time, mtime time.Time) error { return syscall.EROFS }
