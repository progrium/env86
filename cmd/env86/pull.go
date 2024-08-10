package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"tractor.dev/toolkit-go/engine/cli"
)

func pullCmd() *cli.Command {
	cmd := &cli.Command{
		Usage: "pull <repo>[@<tag>]",
		Short: "",
		Args:  cli.MinArgs(1),
		Run: func(ctx *cli.Context, args []string) {
			if strings.HasPrefix(args[0], "http://") || strings.HasPrefix(args[0], "https://") {
				log.Fatal("malformed image path. protocols are not supported")
			}
			if !strings.HasPrefix(args[0], "github.com") {
				log.Fatal("only github repositories are supported at the moment")
			}
			parts := strings.Split(args[0], "@")
			repo := parts[0]
			imageBase := strings.TrimSuffix(path.Base(repo), "-env86")
			tag := "latest"
			if len(parts) > 1 {
				tag = parts[1]
			}

			exists, imageDst := globalImage(args[0])
			if exists {
				log.Fatal("image already exists")
			}

			// fail if repo doesn't exist (trying also with -env86 suffix)
			repoURL := fmt.Sprintf("https://%s", repo)
			resp, err := http.Get(repoURL)
			if err != nil {
				log.Fatal(err)
			}
			if resp.StatusCode == 404 {
				repoURL = fmt.Sprintf("https://%s-env86", repo)
				resp, err = http.Get(repoURL)
				if err != nil {
					log.Fatal(err)
				}
				if resp.StatusCode == 404 {
					log.Fatal("image repo does not exist:", repoURL)
				}
			}

			// fail if repo releases don't exist
			latestReleaseURL := fmt.Sprintf("%s/releases/latest", repoURL)
			resp, err = http.Get(latestReleaseURL)
			if err != nil {
				log.Fatal(err)
			}
			if resp.Request.URL.String() == fmt.Sprintf("%s/releases", repoURL) {
				log.Fatal("image repo has no releases")
			}

			// fail if specific release doesn't exist
			if tag != "latest" {
				releaseURL := fmt.Sprintf("%s/releases/%s", repoURL, tag)
				resp, err = http.Get(releaseURL)
				if err != nil {
					log.Fatal(err)
				}
				if resp.StatusCode == 404 {
					log.Fatal("image repo tag does not exist:", releaseURL)
				}
			} else {
				tag = path.Base(resp.Request.URL.Path)
			}
			if strings.HasSuffix(imageDst, "/local") {
				imageDst = path.Join(strings.TrimSuffix(imageDst, "/local"), tag)
			}

			downloadURL := fmt.Sprintf("%s/releases/download/%s/%s-%s.tgz", repoURL, tag, imageBase, tag)
			resp, err = http.Get(downloadURL)
			if err != nil {
				log.Fatal(err)
			}
			if resp.StatusCode != 200 {
				log.Fatal("unexpected status code fetching image repo tag asset: ", resp.StatusCode, downloadURL)
			}
			defer resp.Body.Close()

			tmpFile, err := os.CreateTemp("", "env86-pull")
			if err != nil {
				log.Fatal(err)
			}
			defer os.Remove(tmpFile.Name())

			_, err = io.Copy(tmpFile, resp.Body)
			if err != nil {
				log.Fatal(err)
			}
			_, err = tmpFile.Seek(0, 0)
			if err != nil {
				log.Fatal(err)
			}
			imageUnzipped, err := gzip.NewReader(tmpFile)
			if err != nil {
				log.Fatal(err)
			}
			defer imageUnzipped.Close()

			imageTar := tar.NewReader(imageUnzipped)

			if err := os.MkdirAll(imageDst, 0755); err != nil {
				log.Fatal(err)
			}

			for {
				header, err := imageTar.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					log.Fatal(err)
				}

				path := filepath.Join(imageDst, header.Name)

				switch header.Typeflag {
				case tar.TypeDir:
					if err := os.MkdirAll(path, os.FileMode(header.Mode)); err != nil {
						log.Fatal(err)
					}
				case tar.TypeReg:
					outFile, err := os.Create(path)
					if err != nil {
						log.Fatal(err)
					}
					defer outFile.Close()

					if _, err := io.Copy(outFile, imageTar); err != nil {
						log.Fatal(err)
					}
				default:
					log.Printf("Unknown type: %v in %s\n", header.Typeflag, header.Name)
				}
			}

			// TODO: set latest symlink if specified or implied tag was latest

		},
	}
	return cmd
}
