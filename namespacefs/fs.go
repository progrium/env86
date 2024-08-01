package namespacefs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"tractor.dev/toolkit-go/engine/fs"
	"tractor.dev/toolkit-go/engine/fs/fsutil"
)

// move into fs
func OpenFile(fsys fs.FS, name string, flag int, perm os.FileMode) (fs.File, error) {
	fsopenfile, ok := fsys.(interface {
		OpenFile(name string, flag int, perm os.FileMode) (fs.File, error)
	})
	if !ok {
		fsopen, ok2 := fsys.(interface {
			Open(name string) (fs.File, error)
		})
		if flag == os.O_RDONLY && perm == 0 && ok2 {
			return fsopen.Open(name)
		}
		return nil, fmt.Errorf("unable to openfile on fs")
	}
	return fsopenfile.OpenFile(name, flag, perm)
}

type binding struct {
	fsys       fs.FS
	mountPoint string
}

// Work in progress FS to implement Plan9-style binding.
// Right now works more like a mountable FS focusing on
// merging mounts into dirs
type FS struct {
	binds []binding
}

func New() *FS {
	return &FS{binds: make([]binding, 0, 1)}
}

func (host *FS) Mount(fsys fs.FS, dirPath string) error {
	dirPath = cleanPath(dirPath)

	// if found, _ := host.isPathInMount(dirPath); found {
	// 	return &fs.PathError{Op: "mount", Path: dirPath, Err: fs.ErrExist}
	// }

	host.binds = append([]binding{{fsys: fsys, mountPoint: dirPath}}, host.binds...)
	return nil
}

func (host *FS) Unmount(path string) error {
	path = cleanPath(path)
	for i, m := range host.binds {
		if path == m.mountPoint {
			host.binds = remove(host.binds, i)
			return nil
		}
	}

	return &fs.PathError{Op: "unmount", Path: path, Err: fs.ErrInvalid}
}

func remove(s []binding, i int) []binding {
	s[i] = s[len(s)-1]
	return s[:len(s)-1]
}

func (host *FS) isPathInMount(path string) (bool, *binding) {
	for i, m := range host.binds {
		if strings.HasPrefix(path, m.mountPoint) || m.mountPoint == "." {
			return true, &host.binds[i]
		}
	}
	return false, nil
}

func cleanPath(p string) string {
	return filepath.Clean(strings.TrimLeft(p, "/\\"))
}

func trimMountPoint(path string, mntPoint string) string {
	result := strings.TrimPrefix(path, mntPoint)
	result = strings.TrimPrefix(result, string(filepath.Separator))

	if result == "" {
		return "."
	} else {
		return result
	}
}

func (host *FS) Chmod(name string, mode fs.FileMode) error {
	name = cleanPath(name)

	if found, mount := host.isPathInMount(name); found {
		chmodableFS, ok := mount.fsys.(interface {
			Chmod(name string, mode fs.FileMode) error
		})
		if ok {
			return chmodableFS.Chmod(trimMountPoint(name, mount.mountPoint), mode)
		}
	}

	return &fs.PathError{Op: "chmod", Path: name, Err: errors.ErrUnsupported}
}

func (host *FS) Chown(name string, uid, gid int) error {
	name = cleanPath(name)

	if found, mount := host.isPathInMount(name); found {
		chownableFS, ok := mount.fsys.(interface {
			Chown(name string, uid, gid int) error
		})
		if ok {
			return chownableFS.Chown(trimMountPoint(name, mount.mountPoint), uid, gid)
		}
	}

	return &fs.PathError{Op: "chown", Path: name, Err: errors.ErrUnsupported}
}

func (host *FS) Chtimes(name string, atime time.Time, mtime time.Time) error {
	name = cleanPath(name)

	if found, mount := host.isPathInMount(name); found {
		chtimesableFS, ok := mount.fsys.(interface {
			Chtimes(name string, atime time.Time, mtime time.Time) error
		})
		if ok {
			return chtimesableFS.Chtimes(trimMountPoint(name, mount.mountPoint), atime, mtime)
		}
	}

	return &fs.PathError{Op: "chtimes", Path: name, Err: errors.ErrUnsupported}
}

func (host *FS) Create(name string) (fs.File, error) {
	name = cleanPath(name)

	if found, mount := host.isPathInMount(name); found {
		createableFS, ok := mount.fsys.(interface {
			Create(name string) (fs.File, error)
		})
		if ok {
			return createableFS.Create(trimMountPoint(name, mount.mountPoint))
		}
	}

	return nil, &fs.PathError{Op: "create", Path: name, Err: errors.ErrUnsupported}
}

func (host *FS) Mkdir(name string, perm fs.FileMode) error {
	name = cleanPath(name)

	if found, mount := host.isPathInMount(name); found {
		mkdirableFS, ok := mount.fsys.(interface {
			Mkdir(name string, perm fs.FileMode) error
		})
		if ok {
			return mkdirableFS.Mkdir(trimMountPoint(name, mount.mountPoint), perm)
		}
	}

	return &fs.PathError{Op: "mkdir", Path: name, Err: errors.ErrUnsupported}
}

func (host *FS) MkdirAll(name string, perm fs.FileMode) error {
	name = cleanPath(name)

	if found, mount := host.isPathInMount(name); found {
		mkdirableFS, ok := mount.fsys.(interface {
			MkdirAll(path string, perm fs.FileMode) error
		})
		if ok {
			return mkdirableFS.MkdirAll(trimMountPoint(name, mount.mountPoint), perm)
		}
	}

	return &fs.PathError{Op: "mkdirAll", Path: name, Err: errors.ErrUnsupported}
}

func (host *FS) Open(name string) (fs.File, error) {
	return host.OpenFile(name, os.O_RDONLY, 0)
}

func (host *FS) OpenFile(name string, flag int, perm fs.FileMode) (fs.File, error) {
	name = cleanPath(name)
	if found, mount := host.isPathInMount(name); found {
		f, err := OpenFile(mount.fsys, trimMountPoint(name, mount.mountPoint), flag, perm)
		if err != nil {
			return nil, err
		}
		var mounts []fs.FileInfo
		for b, m := range host.mountsAtPath(name) {
			if b.mountPoint == mount.mountPoint {
				continue
			}
			mf, err := m.Open(".")
			if err != nil {
				return nil, err
			}
			s, err := mf.Stat()
			if err != nil {
				return nil, err
			}
			mounts = append(mounts, &renamedFileInfo{FileInfo: s, name: filepath.Base(b.mountPoint)})
		}
		if len(mounts) == 0 {
			return f, nil
		}
		return &extraDirsFile{File: f, Dirs: mounts}, nil
	}
	return nil, &fs.PathError{Op: "open", Path: name, Err: errors.ErrUnsupported}
}

type renamedFileInfo struct {
	fs.FileInfo
	name string
}

func (fi *renamedFileInfo) Name() string {
	return fi.name
}

func (host *FS) mountsAtPath(name string) (b map[binding]fs.FS) {
	b = make(map[binding]fs.FS)
	for _, m := range host.binds {
		if filepath.Dir(m.mountPoint) == name {
			b[m] = m.fsys
		}
	}
	return
}

type removableFS interface {
	fs.FS
	Remove(name string) error
}

// func (host *FS) Remove(name string) error {
// 	name = cleanPath(name)

// 		if name == mount.mountPoint {
// 			return &fs.PathError{Op: "remove", Path: name, Err: syscall.EBUSY}
// 		}

// 	if removableFS, ok := fsys.(removableFS); ok {
// 		return removableFS.Remove(trimMountPoint(name, mount.mountPoint))
// 	} else {
// 		return &fs.PathError{Op: "remove", Path: name, Err: errors.ErrUnsupported}
// 	}
// }

// func (host *FS) RemoveAll(path string) error {
// 	path = cleanPath(path)

// 	if found, mount := host.isPathInMount(path); found {
// 		if path == mount.mountPoint {
// 			return &fs.PathError{Op: "removeAll", Path: path, Err: syscall.EBUSY}
// 		}
// 	} else {
// 		fsys = host.MutableFS
// 		// check if path contains any mountpoints, and call a custom removeAll
// 		// if it does.
// 		var mntPoints []string
// 		for _, m := range host.binds {
// 			if path == "." || strings.HasPrefix(m.mountPoint, path) {
// 				mntPoints = append(mntPoints, m.mountPoint)
// 			}
// 		}

// 		if len(mntPoints) > 0 {
// 			return removeAll(host, path, mntPoints)
// 		}
// 	}

// 	rmAllFS, ok := fsys.(interface {
// 		RemoveAll(path string) error
// 	})
// 	if !ok {
// 		if rmFS, ok := fsys.(removableFS); ok {
// 			return removeAll(rmFS, path, nil)
// 		} else {
// 			return &fs.PathError{Op: "removeAll", Path: path, Err: errors.ErrUnsupported}
// 		}
// 	}
// 	return rmAllFS.RemoveAll(trimMountPoint(path, mount.mountPoint))
// }

// RemoveAll removes path and any children it contains. It removes everything
// it can but returns the first error it encounters. If the path does not exist,
// RemoveAll returns nil (no error). If there is an error, it will be of type *PathError.
// Additionally, this function errors if attempting to remove a mountpoint.
func removeAll(fsys removableFS, path string, mntPoints []string) error {
	path = filepath.Clean(path)

	if exists, err := fsutil.Exists(fsys, path); !exists || err != nil {
		return err
	}

	return rmRecurse(fsys, path, mntPoints)

}

func rmRecurse(fsys removableFS, path string, mntPoints []string) error {
	if mntPoints != nil && slices.Contains(mntPoints, path) {
		return &fs.PathError{Op: "remove", Path: path, Err: syscall.EBUSY}
	}

	isdir, dirErr := fsutil.IsDir(fsys, path)
	if dirErr != nil {
		return dirErr
	}

	if isdir {
		if entries, err := fs.ReadDir(fsys, path); err == nil {
			for _, entry := range entries {
				entryPath := filepath.Join(path, entry.Name())

				if err := rmRecurse(fsys, entryPath, mntPoints); err != nil {
					return err
				}

				if err := fsys.Remove(entryPath); err != nil {
					return err
				}
			}
		} else {
			return err
		}
	}

	return fsys.Remove(path)
}

// func (host *FS) Rename(oldname, newname string) error {
// 	oldname = cleanPath(oldname)
// 	newname = cleanPath(newname)

// 	// error if both paths aren't in the same filesystem
// 	if found, oldMount := host.isPathInMount(oldname); found {
// 		if found, newMount := host.isPathInMount(newname); found {
// 			if oldMount != newMount {
// 				return &fs.PathError{Op: "rename", Path: oldname + " -> " + newname, Err: syscall.EXDEV}
// 			}

// 			if oldname == oldMount.mountPoint || newname == newMount.mountPoint {
// 				return &fs.PathError{Op: "rename", Path: oldname + " -> " + newname, Err: syscall.EBUSY}
// 			}

// 			fsys = newMount.fsys
// 			mount.mountPoint = newMount.mountPoint
// 		} else {
// 			return &fs.PathError{Op: "rename", Path: oldname + " -> " + newname, Err: syscall.EXDEV}
// 		}
// 	} else {
// 		if found, _ := host.isPathInMount(newname); found {
// 			return &fs.PathError{Op: "rename", Path: oldname + " -> " + newname, Err: syscall.EXDEV}
// 		}

// 		fsys = host.MutableFS
// 	}

// 	renameableFS, ok := fsys.(interface {
// 		Rename(oldname, newname string) error
// 	})
// 	if !ok {
// 		return &fs.PathError{Op: "rename", Path: oldname + " -> " + newname, Err: errors.ErrUnsupported}
// 	}
// 	return renameableFS.Rename(trimMountPoint(oldname, mount.mountPoint), trimMountPoint(newname, mount.mountPoint))
// }
