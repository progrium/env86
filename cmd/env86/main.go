package main

import (
	"context"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/progrium/env86/fsutil"

	"tractor.dev/toolkit-go/desktop"
	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/toolkit-go/engine/fs"
)

var Version = "dev"

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	root := &cli.Command{
		Version: Version,
		Usage:   "env86",
		Long:    `env86 manages v86 emulated virtual machines`,
	}

	root.AddCommand(bootCmd())
	root.AddCommand(prepareCmd())
	root.AddCommand(networkCmd())
	root.AddCommand(serveCmd())
	root.AddCommand(createCmd())
	root.AddCommand(assetsCmd())
	root.AddCommand(runCmd())
	root.AddCommand(pullCmd())

	desktop.Start(func() {
		if err := cli.Execute(context.Background(), root, os.Args[1:]); err != nil {
			log.Fatal(err)
		}
		desktop.Stop()
	})
}

func env86Path() string {
	path := os.Getenv("ENV86_PATH")
	if path == "" {
		if runtime.GOOS == "windows" {
			path = filepath.Join(os.Getenv("APPDATA"), "env86")
		} else {
			usr, _ := user.Current()
			path = usr.HomeDir + "/.env86"
		}
	}
	return path
}

// globalImage resolves a pathspec to a global image path
// On Unix-like systems:
// github.com/progrium/alpine@latest => ~/.env86/github.com/progrium/alpine/3.18
// On Windows:
// github.com/progrium/alpine@latest => %APPDATA%\env86\github.com\progrium\alpine\3.18
func globalImage(pathspec string) (bool, string) {
	parts := strings.Split(pathspec, "@")
	image := strings.TrimSuffix(parts[0], "-env86")
	tag := "latest"
	if len(parts) > 1 {
		tag = parts[1]
	}

	path := filepath.Join(env86Path(), image, tag)
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		path = resolved
	}
	ok, err := fs.Exists(fsutil.RootFS(path), fsutil.RootFSRelativePath(path))
	if err != nil {
		log.Fatal(err)
	}
	if ok {
		return true, path
	}

	// if tag explicitly specified and not found
	if len(parts) > 1 {
		return false, path
	}
	// if no tag specified and latest not found, try local
	path = filepath.Join(env86Path(), image, "local")
	ok, err = fs.Exists(fsutil.RootFS(path), fsutil.RootFSRelativePath(path))
	if err != nil {
		log.Fatal(err)
	}
	if ok {
		return true, path
	}
	return false, path
}
