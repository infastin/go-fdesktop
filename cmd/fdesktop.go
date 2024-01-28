package main

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"strings"

	"github.com/adrg/xdg"
	"github.com/alecthomas/kong"
	"github.com/infastin/go-fdesktop"
)

var cli struct {
	ShowID    bool   `optional:"" short:"i" help:"If specified AppID will be printed"`
	ShowPath  bool   `optional:"" short:"p" help:"If specified Path will be printed"`
	ShowName  bool   `optional:"" short:"n" help:"If specified Name will be printed"`
	Delimiter string `optional:"" short:"d" default:":" help:"Delimiter for shown attributes"`
	Null      bool   `optional:"" short:"0" help:"Separate results by the null byte"`
}

func main() {
	kong.Parse(&cli)

	entries := []*fdesktop.Entry{}

	for _, dir := range xdg.DataDirs {
		appDir := path.Join(dir, "applications")
		if stat, err := os.Stat(appDir); err != nil || !stat.IsDir() {
			continue
		}

		fileSystem := os.DirFS(appDir)
		err := fs.WalkDir(fileSystem, ".", func(filepath string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if filepath == "." {
				return nil
			}

			if d.IsDir() {
				return fs.SkipDir
			}

			if !strings.HasSuffix(filepath, ".desktop") {
				return nil
			}

			appId := filepath[0 : len(filepath)-len(".desktop")]
			entryPath := path.Join(appDir, filepath)

			file, err := fileSystem.Open(filepath)
			if err != nil {
				return err
			}
			defer file.Close()

			entry := fdesktop.NewEntry(appId, entryPath)
			err = entry.Decode(file)
			if err != nil {
				return fmt.Errorf("file %s: %v", entryPath, err)
			}

			entries = append(entries, entry)
			return nil
		})

		if err != nil {
			log.Fatal(err)
		}
	}

	var b strings.Builder

	for _, e := range entries {
		if e.TryNoDisplay() {
			continue
		}

		if cli.ShowID {
			b.WriteString(e.AppId)
		}

		if cli.ShowID && (cli.ShowName || cli.ShowPath) {
			b.WriteString(cli.Delimiter)
		}

		if cli.ShowName {
			b.WriteString(e.Name())
		}

		if cli.ShowName && cli.ShowPath {
			b.WriteString(cli.Delimiter)
		}

		if cli.ShowPath {
			b.WriteString(e.Path)
		}

		if cli.Null {
			b.WriteRune(0)
		} else {
			b.WriteRune('\n')
		}

		fmt.Print(b.String())
		b.Reset()
	}
}
