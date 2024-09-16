package main

import (
	"errors"

	"tractor.dev/toolkit-go/engine/fs"
)

type Entry struct {
	IsDir bool
	Ctime int
	Mtime int
	Size  int
	Name  string
}

func (api *API) Stat(path string) (*Entry, error) {
	fi, err := fs.Stat(api.FS, path)
	if err != nil {
		return nil, err
	}
	return &Entry{
		Name:  fi.Name(),
		Mtime: int(fi.ModTime().Unix()),
		IsDir: fi.IsDir(),
		Ctime: 0,
		Size:  int(fi.Size()),
	}, nil
}

func (api *API) ReadFile(path string) ([]byte, error) {
	return fs.ReadFile(api.FS, path)
}

func (api *API) ReadDir(path string) ([]Entry, error) {
	dir, err := fs.ReadDir(api.FS, path)
	if err != nil {
		return nil, err
	}
	var entries []Entry
	for _, e := range dir {
		fi, _ := e.Info()
		entries = append(entries, Entry{
			Name:  fi.Name(),
			Mtime: int(fi.ModTime().Unix()),
			IsDir: fi.IsDir(),
			Ctime: 0,
			Size:  int(fi.Size()),
		})
	}
	return entries, nil
}

func (api *API) WriteFile(path string, data []byte) error {
	return fs.WriteFile(api.FS, path, data, 0644)
}

func (api *API) MakeDir(path string) error {
	return fs.MkdirAll(api.FS, path, 0744)
}

func (api *API) RemoveAll(path string) error {
	rfs, ok := api.FS.(interface {
		RemoveAll(path string) error
	})
	if !ok {
		return errors.ErrUnsupported
	}
	return rfs.RemoveAll(path)
}

func (api *API) Rename(path, newpath string) error {
	rfs, ok := api.FS.(interface {
		Rename(oldname, newname string) error
	})
	if !ok {
		return errors.ErrUnsupported
	}
	return rfs.Rename(path, newpath)
}
