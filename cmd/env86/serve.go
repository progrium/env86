package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/progrium/env86"
	"github.com/progrium/env86/assets"
	"github.com/progrium/env86/namespacefs"

	"tractor.dev/toolkit-go/engine/cli"
)

func serveCmd() *cli.Command {
	cmd := &cli.Command{
		Usage: "serve <image>",
		Short: "serve a VM and debug console over HTTP",
		Args:  cli.MinArgs(1),
		Run: func(ctx *cli.Context, args []string) {
			image, err := env86.LoadImage(args[0])
			if err != nil {
				log.Fatal(err)
			}

			preparedImage, err := image.Prepare()
			if err != nil {
				log.Fatal(err)
			}

			fsys := namespacefs.New()
			if err := fsys.Mount(assets.Dir, "/"); err != nil {
				log.Fatal(err)
			}
			if err := fsys.Mount(preparedImage, "image"); err != nil {
				log.Fatal(err)
			}

			bundle, err := assets.BundleJavaScript()
			if err != nil {
				log.Fatal(err)
			}

			mux := http.NewServeMux()
			mux.Handle("/", http.FileServerFS(fsys))
			mux.Handle("/env86.min.js", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Add("content-type", "text/javascript")
				io.Copy(w, bytes.NewBuffer(bundle))
			}))

			fmt.Println("serving: http://localhost:9999/")
			http.ListenAndServe(":9999", mux)
		},
	}
	return cmd
}
