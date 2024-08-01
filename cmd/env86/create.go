package main

import (
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/toolkit-go/engine/fs"
)

func createCmd() *cli.Command {
	var (
		dir    string
		docker string
	)
	cmd := &cli.Command{
		Usage: "create <image>",
		Short: "create an image from directory or using Docker",
		Args:  cli.MinArgs(1),
		Run: func(ctx *cli.Context, args []string) {
			imagePath, err := filepath.Abs(args[0])
			if err != nil {
				log.Fatal(err)
			}
			exists, err := fs.Exists(os.DirFS("/"), strings.TrimPrefix(imagePath, "/"))
			if err != nil {
				log.Fatal(err)
			}
			if exists {
				log.Fatal("image filepath already exists")
			}

			if dir != "" {
				dir, err := filepath.Abs(dir)
				if err != nil {
					log.Fatal(err)
				}
				isDir, err := fs.IsDir(os.DirFS("/"), strings.TrimPrefix(dir, "/"))
				if err != nil {
					log.Fatal(err)
				}
				if !isDir {
					log.Fatal("specified dir does not exist")
				}
			}

			// if docker image or dockerfile specified, generate dir from docker export
			if docker != "" {
				imageName := docker

				var err error
				docker, err = filepath.Abs(docker)
				if err != nil {
					log.Fatal(err)
				}
				isDockerfile, err := fs.Exists(os.DirFS("/"), strings.TrimPrefix(docker, "/"))
				if err != nil {
					log.Fatal(err)
				}
				if isDockerfile {
					ctxDir := filepath.Dir(docker)
					imageName = "env86-build"
					run(ctxDir, "docker", "build", "--platform=linux/386", "-t", imageName, "-f", docker, ".")
				}

				outDir, err := os.MkdirTemp("", "env86-create")
				if err != nil {
					log.Fatal(err)
				}
				defer os.RemoveAll(outDir)

				run(outDir, "docker", "create", "--platform=linux/386", "--name=env86-create", imageName)
				run(outDir, "docker", "export", "env86-create", "-o", "fs.tar")
				run(outDir, "docker", "rm", "env86-create")
				os.MkdirAll(filepath.Join(outDir, "fs"), 0755)
				run(outDir, "tar", "-xvf", "fs.tar", "-C", "fs")
				run(outDir, "sh", "-c", "chmod -R +r fs")
				os.RemoveAll(filepath.Join(outDir, "fs.tar"))
				os.RemoveAll(filepath.Join(outDir, "fs/.dockerenv"))

				dir = filepath.Join(outDir, "fs")
			}

			if dir == "" {
				log.Fatal("nothing to create from")
			}

			if err := os.MkdirAll(imagePath, 0755); err != nil {
				log.Fatal(err)
			}

			GenerateIndex(filepath.Join(imagePath, "fs.json"), dir, nil)
			CopyToSha256(dir, filepath.Join(imagePath, "fs"))

			imageConfig := map[string]any{
				"cmdline": "rw root=host9p rootfstype=9p rootflags=trans=virtio,cache=loose modules=virtio_pci console=ttyS0 console=tty1",
			}
			b, err := json.MarshalIndent(imageConfig, "", "  ")
			if err != nil {
				log.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(imagePath, "image.json"), b, 0644); err != nil {
				log.Fatal(err)
			}

		},
	}
	cmd.Flags().StringVar(&dir, "from-dir", "", "make image from directory root")
	cmd.Flags().StringVar(&docker, "from-docker", "", "make image from Docker image or Dockerfile")
	return cmd
}

func run(dir, name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		os.Stderr.Write(out)
		log.Fatal(err)
	}
}
