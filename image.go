package env86

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/progrium/env86/assets"
	"github.com/progrium/env86/fsutil"
	"github.com/progrium/env86/tarfs"

	"github.com/klauspost/compress/zstd"
	"tractor.dev/toolkit-go/engine/fs"
	"tractor.dev/toolkit-go/engine/fs/memfs"
	"tractor.dev/toolkit-go/engine/fs/osfs"
)

type Image struct {
	FS fs.FS
}

func LoadImageReader(r io.Reader) (*Image, error) {
	imageUnzipped, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer imageUnzipped.Close()

	return &Image{FS: tarfs.New(tar.NewReader(imageUnzipped))}, nil
}

func LoadImage(path string) (*Image, error) {
	imagePath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	isDir, err := fs.IsDir(fsutil.RootFS(imagePath), fsutil.RootFSRelativePath(imagePath))
	if err != nil {
		return nil, err
	}

	if isDir {
		return &Image{FS: osfs.Dir(imagePath)}, nil
	}

	imageFile, err := os.Open(imagePath)
	if err != nil {
		return nil, err
	}
	defer imageFile.Close()
	return LoadImageReader(imageFile)
}

func (i *Image) Config() (Config, error) {
	coldboot := !i.HasInitialState()
	if ok, _ := fs.Exists(i.FS, "image.json"); !ok {
		return Config{}, fmt.Errorf("no image.json found in image")
	}
	v86conf, err := i.v86Config()
	if err != nil {
		return Config{}, err
	}
	return Config{
		V86Config: v86conf,
		ColdBoot:  coldboot,
	}, nil
}

func (i *Image) v86Config() (V86Config, error) {
	b, err := fs.ReadFile(i.FS, "image.json")
	if err != nil {
		return V86Config{}, err
	}
	var v86conf V86Config
	if err := json.Unmarshal(b, &v86conf); err != nil {
		return V86Config{}, err
	}
	return v86conf, nil
}

func (i *Image) HasInitialState() bool {
	b, _ := fs.Exists(i.FS, "initial.state")
	return b || i.hasCompressedInitialState()
}

func (i *Image) hasCompressedInitialState() (b bool) {
	b, _ = fs.Exists(i.FS, "initial.state.zst")
	return
}

func (i *Image) InitialStateConfig() *ImageConfig {
	if !i.HasInitialState() {
		return nil
	}
	if i.hasCompressedInitialState() {
		fi, err := fs.Stat(i.FS, "initial.state.zst")
		if err != nil {
			panic(err)
		}
		return &ImageConfig{
			URL:  "/image/initial.state.zst",
			Size: int(fi.Size()),
		}
	}
	return &ImageConfig{
		URL: "/image/initial.state",
	}
}

func (i *Image) SaveInitialState(r io.Reader) error {
	// TODO: write new tgz if loaded from tgz

	shouldCompress := i.hasCompressedInitialState() // || !i.HasInitialState()
	if shouldCompress {
		var buf bytes.Buffer
		// BUG: this implementation doesn't seem to be compatible with v86's decompressor!
		enc, err := zstd.NewWriter(&buf)
		if err != nil {
			return err
		}
		defer enc.Close()
		_, err = io.Copy(enc, r)
		if err != nil {
			return err
		}
		return fs.WriteFile(i.FS, "initial.state.zst", buf.Bytes(), 0644)
	}

	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	return fs.WriteFile(i.FS, "initial.state", b, 0644)
}

func (i *Image) Prepare() (fs.FS, error) {
	fsys := memfs.New()
	conf, err := i.v86Config()
	if err != nil {
		return nil, err
	}

	if err := fsutil.CopyFS(i.FS, ".", fsys, "."); err != nil {
		return nil, err
	}

	if !i.HasInitialState() {
		conf.BIOS = &ImageConfig{URL: "./seabios.bin"}
		if err := fsutil.CopyFS(assets.Dir, "seabios.bin", fsys, "seabios.bin"); err != nil {
			return nil, err
		}

		conf.VGABIOS = &ImageConfig{URL: "./vgabios.bin"}
		if err := fsutil.CopyFS(assets.Dir, "vgabios.bin", fsys, "vgabios.bin"); err != nil {
			return nil, err
		}

		conf.Initrd = &ImageConfig{URL: "./initramfs.bin"}
		if err := fsutil.CopyFS(assets.Dir, "initramfs.bin", fsys, "initramfs.bin"); err != nil {
			return nil, err
		}

		conf.BZImage = &ImageConfig{URL: "./vmlinuz.bin"}
		if err := fsutil.CopyFS(assets.Dir, "vmlinuz.bin", fsys, "vmlinuz.bin"); err != nil {
			return nil, err
		}

		if err := writeConfig(fsys, "image.json", conf); err != nil {
			return nil, err
		}
	}

	if !i.HasInitialState() || i.hasCompressedInitialState() {
		return fsys, nil
	}

	fsys.MkdirAll("state", 0755)
	n, err := splitFile(fsys, "initial.state", "state", 10*1024*1024)
	if err != nil {
		return nil, err
	}
	if err := fsys.Remove("initial.state"); err != nil {
		return nil, err
	}

	conf.InitialStateParts = n
	if err := writeConfig(fsys, "image.json", conf); err != nil {
		return nil, err
	}

	return fsys, nil
}

func writeConfig(fsys fs.FS, filename string, cfg V86Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return fs.WriteFile(fsys, filename, data, 0644)
}

func splitFile(fsys fs.FS, filename string, outputDir string, bytesPerFile int) (int, error) {
	f, ok := fsys.(interface {
		Create(name string) (fs.File, error)
		Chmod(name string, mode fs.FileMode) error
	})
	if !ok {
		return 0, fs.ErrPermission
	}
	file, err := fsys.Open(filename)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	baseName := path.Base(filename)

	buffer := make([]byte, bytesPerFile)
	part := 0
	for {
		bytesRead, rerr := file.Read(buffer)
		if bytesRead == 0 {
			break
		}

		outputFilename := fmt.Sprintf("%s.%d", baseName, part)
		outputFile, err := f.Create(path.Join(outputDir, outputFilename))
		if err != nil {
			return 0, err
		}

		wf, ok := outputFile.(interface {
			Write(p []byte) (n int, err error)
		})
		if !ok {
			return 0, fs.ErrPermission
		}

		_, err = wf.Write(buffer[:bytesRead])
		if err != nil {
			return 0, err
		}
		outputFile.Close()
		if err := f.Chmod(path.Join(outputDir, outputFilename), 0644); err != nil {
			return 0, err
		}

		if rerr == io.EOF {
			break
		}
		part++
	}
	return part, nil
}
