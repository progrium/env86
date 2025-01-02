package fsutil

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"

	"tractor.dev/toolkit-go/engine/fs"
)

// CopyAll recursively copies the file, directory or symbolic link at src
// to dst. The destination must not exist. Symbolic links are not
// followed.
//
// If the copy fails half way through, the destination might be left
// partially written.
func CopyAll(fsys fs.FS, src, dst string) error {
	return CopyFS(fsys, src, fsys, dst)
}

func CopyFS(srcFS fs.FS, srcPath string, dstFS fs.FS, dstPath string) error {
	mfs, ok := dstFS.(fs.MutableFS)
	if !ok {
		return errors.New("not a mutable filesystem")
	}
	srcInfo, srcErr := fs.Stat(srcFS, srcPath)
	if srcErr != nil {
		return srcErr
	}
	dstInfo, dstErr := fs.Stat(dstFS, dstPath)
	if dstErr == nil && !dstInfo.IsDir() {
		return fmt.Errorf("will not overwrite %q", dstPath)
	}
	switch mode := srcInfo.Mode(); mode & fs.ModeType {
	// case os.ModeSymlink:
	// 	return copySymLink(src, dst)
	case os.ModeDir:
		return copyDir(srcFS, srcPath, mfs, dstPath, mode)
	case 0:
		return copyFile(srcFS, srcPath, mfs, dstPath, mode)
	default:
		return fmt.Errorf("cannot copy file with mode %v", mode)
	}
}

// func copySymLink(src, dst string) error {
// 	target, err := os.Readlink(src)
// 	if err != nil {
// 		return err
// 	}
// 	return os.Symlink(target, dst)
// }

func copyFile(srcFS fs.FS, srcPath string, dstFS fs.MutableFS, dstPath string, mode fs.FileMode) error {
	srcf, err := srcFS.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcf.Close()
	dstf, err := dstFS.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode.Perm())
	if err != nil {
		return err
	}
	defer dstf.Close()
	// Make the actual permissions match the source permissions
	// even in the presence of umask.
	if err := dstFS.Chmod(dstPath, mode.Perm()); err != nil {
		return fmt.Errorf("chmod1: %w", err)
	}
	wdstf, ok := dstf.(io.Writer)
	if !ok {
		return fmt.Errorf("cannot copy %q to %q: dst not writable", srcPath, dstPath)
	}
	if _, err := io.Copy(wdstf, srcf); err != nil {
		return fmt.Errorf("cannot copy %q to %q: %v", srcPath, dstPath, err)
	}
	return nil
}

func copyDir(srcFS fs.FS, srcPath string, dstFS fs.MutableFS, dstPath string, mode fs.FileMode) error {
	srcf, err := srcFS.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcf.Close()
	if mode&0500 == 0 {
		// The source directory doesn't have write permission,
		// so give the new directory write permission anyway
		// so that we have permission to create its contents.
		// We'll make the permissions match at the end.
		mode |= 0500
	}
	if err := dstFS.MkdirAll(dstPath, mode.Perm()); err != nil {
		return err
	}
	entries, err := fs.ReadDir(srcFS, srcPath)
	if err != nil {
		return fmt.Errorf("error reading directory %q: %v", srcPath, err)
	}
	for _, entry := range entries {
		if err := CopyFS(srcFS, path.Join(srcPath, entry.Name()), dstFS, path.Join(dstPath, entry.Name())); err != nil {
			return err
		}
	}
	if dstPath == "." {
		return nil
	}
	if err := dstFS.Chmod(dstPath, mode.Perm()); err != nil {
		return err
	}
	return nil
}
