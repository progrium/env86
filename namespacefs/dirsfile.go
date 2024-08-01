package namespacefs

import (
	"io"
	"io/fs"
)

type extraDirsFile struct {
	fs.File
	Dirs    []fs.FileInfo
	off     int
	entries []fs.DirEntry
}

func (f *extraDirsFile) ReadDir(c int) (dir []fs.DirEntry, err error) {
	if f.off == 0 {
		f.entries, err = f.File.(fs.ReadDirFile).ReadDir(c)
		if err != nil {
			return nil, err
		}
		for i := 0; i < len(f.Dirs); i++ {
			f.entries = append(f.entries, fs.FileInfoToDirEntry(f.Dirs[i]))
		}
	}
	entries := f.entries[f.off:]

	if c <= 0 {
		return entries, nil
	}

	if len(entries) == 0 {
		return nil, io.EOF
	}

	if c > len(entries) {
		c = len(entries)
	}

	defer func() { f.off += c }()
	return entries[:c], nil
}
