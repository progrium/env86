package env86

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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

func LoadImage(path string) (*Image, error) {
	imagePath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	isDir, err := fs.IsDir(os.DirFS("/"), strings.TrimPrefix(imagePath, "/"))
	if err != nil {
		return nil, err
	}

	var imageFS fs.FS

	if isDir {
		imageFS = osfs.Dir(imagePath)
	} else {
		imageFile, err := os.Open(imagePath)
		if err != nil {
			return nil, err
		}
		defer imageFile.Close()

		imageUnzipped, err := gzip.NewReader(imageFile)
		if err != nil {
			return nil, err
		}
		defer imageUnzipped.Close()

		imageFS = tarfs.New(tar.NewReader(imageUnzipped))
	}

	return &Image{FS: imageFS}, nil
}

func (i *Image) Config() (Config, error) {
	coldboot := !i.HasInitialState()
	if ok, _ := fs.Exists(i.FS, "image.json"); !ok {
		return Config{
			ColdBoot: coldboot,
		}, nil
	}
	b, err := fs.ReadFile(i.FS, "image.json")
	if err != nil {
		return Config{}, err
	}
	var v86conf V86Config
	if err := json.Unmarshal(b, &v86conf); err != nil {
		return Config{}, err
	}
	return Config{
		V86Config: v86conf,
		ColdBoot:  coldboot,
	}, nil
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

	if err := fsutil.CopyFS(i.FS, ".", fsys, "."); err != nil {
		return nil, err
	}

	fsys.MkdirAll("state", 0755)
	n, err := splitFile(fsys, "initial.state", "state", 10*1024*1024)
	if err != nil {
		return nil, err
	}
	if err := fsys.Remove("initial.state"); err != nil {
		return nil, err
	}

	configIn, err := fs.ReadFile(fsys, "image.json")
	if err != nil {
		return nil, err
	}
	var config map[string]any
	if err := json.Unmarshal(configIn, &config); err != nil {
		return nil, err
	}
	config["initial_state_parts"] = n
	configOut, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := fs.WriteFile(fsys, "image.json", configOut, 0644); err != nil {
		return nil, err
	}

	return fsys, nil
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

	baseName := filepath.Base(filename)

	buffer := make([]byte, bytesPerFile)
	part := 0
	for {
		bytesRead, rerr := file.Read(buffer)
		if bytesRead == 0 {
			break
		}

		outputFilename := fmt.Sprintf("%s.%d", baseName, part)
		outputFile, err := f.Create(filepath.Join(outputDir, outputFilename))
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
		if err := f.Chmod(filepath.Join(outputDir, outputFilename), 0644); err != nil {
			return 0, err
		}

		if rerr == io.EOF {
			break
		}
		part++
	}
	return part, nil
}
