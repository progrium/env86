package assets

import (
	"embed"
	"env86/fsutil"
	"fmt"
	"os"
	"path/filepath"

	"github.com/evanw/esbuild/pkg/api"
	"tractor.dev/toolkit-go/engine/fs"
	"tractor.dev/toolkit-go/engine/fs/osfs"
)

//go:embed *
var assets embed.FS

var Dir = fs.LiveDir(assets)

func BundleJavaScript() ([]byte, error) {
	tmpDir, err := os.MkdirTemp("", "env86-js")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	if err := fsutil.CopyFS(Dir, "env86.js", osfs.New(), filepath.Join(tmpDir, "env86.js")); err != nil {
		return nil, err
	}
	if err := fsutil.CopyFS(Dir, "duplex.min.js", osfs.New(), filepath.Join(tmpDir, "duplex.min.js")); err != nil {
		return nil, err
	}

	result := api.Build(api.BuildOptions{
		EntryPoints:       []string{filepath.Join(tmpDir, "env86.js")},
		Bundle:            true,
		Outdir:            tmpDir,
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		MinifySyntax:      true,
		Format:            api.FormatESModule,
		Platform:          api.PlatformBrowser,
	})
	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("esbuild: %s [%s:%d]",
			result.Errors[0].Text,
			filepath.Base(result.Errors[0].Location.File),
			result.Errors[0].Location.Line)
	}

	libv86, err := fs.ReadFile(Dir, "libv86.js")
	if err != nil {
		return nil, err
	}

	return append(libv86, result.OutputFiles[0].Contents...), nil
}
