package main

import (
	"env86"
	"env86/assets"
	"env86/fsutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/toolkit-go/engine/fs"
	"tractor.dev/toolkit-go/engine/fs/osfs"
)

func prepareCmd() *cli.Command {
	cmd := &cli.Command{
		Usage: "prepare <image> <dir>",
		Short: "prepare a VM for publishing on the web",
		Args:  cli.MinArgs(2),
		Run: func(ctx *cli.Context, args []string) {
			image, err := env86.LoadImage(args[0])
			if err != nil {
				log.Fatal(err)
			}

			dstPath, err := filepath.Abs(args[1])
			if err != nil {
				log.Fatal(err)
			}
			exists, err := fs.DirExists(os.DirFS("/"), strings.TrimLeft(dstPath, "/"))
			if err != nil {
				log.Fatal(err)
			}
			if exists {
				log.Fatal("destination dir already exists")
			}
			os.MkdirAll(dstPath, 0755)

			preparedImage, err := image.Prepare()
			if err != nil {
				log.Fatal(err)
			}

			dst := osfs.Dir(dstPath)
			dst.MkdirAll("image", 0755)
			if err := fsutil.CopyFS(preparedImage, ".", dst, "image"); err != nil {
				log.Fatal(err)
			}

			bundle, err := assets.BundleJavaScript()
			if err != nil {
				log.Fatal(err)
			}
			if err := fs.WriteFile(dst, "env86.min.js", bundle, 0644); err != nil {
				log.Fatal(err)
			}
			if err := fsutil.CopyFS(assets.Dir, "v86.wasm", dst, "v86.wasm"); err != nil {
				log.Fatal(err)
			}
			if err := fsutil.CopyFS(assets.Dir, "index.html", dst, "index.html"); err != nil {
				log.Fatal(err)
			}
		},
	}
	return cmd
}
