package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// v86 tool scripts fs2json.py and copy-to-sha256.py
// ported to go by Michael Hermenault

const (
	VERSION      = 3
	IDX_NAME     = 0
	IDX_SIZE     = 1
	IDX_MTIME    = 2
	IDX_MODE     = 3
	IDX_UID      = 4
	IDX_GID      = 5
	IDX_TARGET   = 6
	IDX_FILENAME = 6
	HASH_LENGTH  = 8
	S_IFLNK      = 0xA000
	S_IFREG      = 0x8000
	S_IFDIR      = 0x4000
)

func GenerateIndex(outFile string, path string, exclude []string) {
	excludes := stringSlice(exclude)
	path = filepath.Clean(path)
	var root []interface{}
	var totalSize int64

	fi, err := os.Stat(path)
	if err != nil {
		log.Fatal(err)
	}

	if fi.IsDir() {
		root, totalSize = indexHandleDir(path, excludes)
	} else {
		f, err := os.Open(path)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()

	}

	result := map[string]interface{}{
		"fsroot":  root,
		"version": VERSION,
		"size":    totalSize,
	}

	// logger.Println("Creating json ...")
	enc := json.NewEncoder(os.Stdout)
	if outFile != "" {
		f, err := os.Create(outFile)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		enc = json.NewEncoder(f)
	}
	enc.Encode(result)
}

func indexHandleDir(path string, excludes []string) ([]interface{}, int64) {
	var totalSize int64
	mainRoot := make([]interface{}, 0)
	filenameToHash := make(map[string]string)

	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("Error accessing path %q: %v\n", filePath, err)
			return nil
		}

		relPath, err := filepath.Rel(path, filePath)
		if err != nil {
			return err
		}

		parts := strings.Split(relPath, string(os.PathSeparator))
		for _, exclude := range excludes {
			if parts[0] == exclude {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		name := parts[len(parts)-1]
		dir := &mainRoot
		if name == "." {
			return nil
		}
		for _, p := range parts[:len(parts)-1] {
			for _, lilD := range *dir {
				lilD, _ := lilD.([]interface{})
				name, ok := lilD[IDX_NAME].(string)
				if ok && name == p {
					newDir, ok := lilD[IDX_TARGET].(*[]interface{})
					if !ok {
						log.Panic("could not cast refrence to slice")
					}
					dir = newDir
					break
				}
			}
		}
		obj := make([]interface{}, 7)

		_, err = os.Lstat(filePath)
		if err != nil {
			log.Fatal(err)
		}

		var UID int
		var GID int
		// if stat, ok := statInfo.Sys().(*syscall.Stat_t); ok {
		// 	UID = int(stat.Uid)
		// 	GID = int(stat.Gid)
		// }

		obj[IDX_NAME] = name
		obj[IDX_SIZE] = info.Size()
		obj[IDX_MTIME] = info.ModTime().Unix()
		obj[IDX_MODE] = int64(info.Mode())
		obj[IDX_UID] = UID // Not available in Go's os.FileInfo
		obj[IDX_GID] = GID // Not available in Go's os.FileInfo

		if info.Mode()&os.ModeSymlink != 0 {
			obj[IDX_MODE] = obj[IDX_MODE].(int64) | S_IFLNK
			target, err := os.Readlink(filePath)
			if err != nil {
				return err
			}
			obj[IDX_TARGET] = target
		} else if info.IsDir() {
			obj[IDX_MODE] = obj[IDX_MODE].(int64) | S_IFDIR
			newDir := make([]interface{}, 0)
			obj[IDX_TARGET] = &newDir
		} else {
			obj[IDX_MODE] = obj[IDX_MODE].(int64) | S_IFREG
			fileHash, err := hashFile(filePath)
			if err != nil {
				return err
			}
			filename := fileHash[:HASH_LENGTH] + ".bin"
			if existing, ok := filenameToHash[filename]; ok {
				if existing != fileHash {
					return fmt.Errorf("collision in short hash (%s and %s)", existing, fileHash)
				}
			}
			filenameToHash[filename] = fileHash
			obj[IDX_FILENAME] = filename
		}

		totalSize += info.Size()
		*dir = append(*dir, obj)

		return nil
	})

	if err != nil {
		log.Fatal(err)
	}
	return mainRoot, totalSize
}

type stringSlice []string

func (s *stringSlice) String() string {
	return strings.Join(*s, ",")
}

func (s *stringSlice) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func hashFile(filename string) (string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()
	return hashFileObject(f), nil
}

func hashFileObject(f io.Reader) string {
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		log.Fatal(err)
	}
	return hex.EncodeToString(h.Sum(nil))
}

//////

func CopyToSha256(fromPath, toPath string) {
	os.MkdirAll(toPath, 0755)
	fromFile, err := os.Open(fromPath)
	if err != nil {
		log.Fatal(err)
	}
	_, err = gzip.NewReader(fromFile)

	fromFile.Close()
	if err != nil {
		handleDir(fromPath, toPath)
		return
	}
	handleTar(fromPath, toPath)

}

func handleTar(fromPath, toPath string) {
	fromFile, err := os.Open(fromPath)
	if err != nil {
		log.Fatal(err)
	}
	defer fromFile.Close()
	gzip, _ := gzip.NewReader(fromFile)

	tarReader := tar.NewReader(gzip)

	for {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			continue
		case tar.TypeReg, tar.TypeLink:
			var buf bytes.Buffer
			tee := io.TeeReader(tarReader, &buf)
			fileHash := hashFileObject(tee)
			absPath := filepath.Join(toPath, fileHash[0:HASH_LENGTH]+".bin")
			if _, err := os.Stat(absPath); err == nil {
				//log.Printf("Exists, skipped %s (%s)\n", absPath, header.Name)
				continue
			}
			copyFileObject(&buf, absPath)
		}
	}
}

func handleDir(fromPath, toPath string) {
	visit := func(path string, di fs.DirEntry, dirError error) error {
		if di.Type() == fs.ModeSymlink || di.Type() == fs.ModeCharDevice || di.Type() == fs.ModeDevice || di.Type() == fs.ModeDevice || di.Type() == fs.ModeSocket || di.Type() == fs.ModeDir {
			return nil
		}
		fromFile, err := os.Open(path)
		if err != nil {
			log.Fatalf("Error reading file %s\n", path)
		}
		defer fromFile.Close()
		fileHash := hashFileObject(fromFile)
		absPath := filepath.Join(toPath, fileHash[0:HASH_LENGTH]+".bin")
		if _, err := os.Stat(absPath); err == nil {
			//log.Printf("Exists, skipped %s (%s)\n", absPath, path)
			return nil
		}
		copyFileContents(path, absPath)
		return nil
	}

	filepath.WalkDir(fromPath, visit)
}

func copyFileContents(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()
	copyFileObject(in, dst)
	return
}

func copyFileObject(src io.Reader, dst string) (err error) {
	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(out, src); err != nil {
		return
	}
	err = out.Sync()
	return
}
