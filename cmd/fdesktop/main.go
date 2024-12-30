package main

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"

	"github.com/adrg/xdg"
	"github.com/alecthomas/kong"
	"github.com/infastin/go-fdesktop"
)

var cli struct {
	ShowID    bool   `optional:"" name:"id" short:"i" help:"Print AppID."`
	ShowPath  bool   `optional:"" name:"path" short:"p" default:"true" negatable:"" help:"Print Path."`
	ShowName  bool   `optional:"" name:"name" short:"n" default:"true" negatable:"" help:"Print Name."`
	Delimiter string `optional:"" short:"d" default:"\t" help:"Delimiter for printed attributes."`
	Null      bool   `optional:"" short:"0" help:"Separate results by the null byte."`
}

func main() {
	kong.Parse(&cli)

	var entries []*fdesktop.Entry
	for _, dir := range xdg.DataDirs {
		appDir := path.Join(dir, "applications")
		if stat, err := os.Stat(appDir); err != nil || !stat.IsDir() {
			continue
		}

		fileSystem := os.DirFS(appDir)
		if err := fs.WalkDir(fileSystem, ".", func(filepath string, d fs.DirEntry, err error) error {
			if err != nil || filepath == "." {
				return err
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
			if err := entry.Decode(file); err != nil {
				return fmt.Errorf("file %s: %v", entryPath, err)
			}

			entries = append(entries, entry)
			return nil
		}); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	var sep string
	if cli.Null {
		sep = "\000"
	} else {
		sep = "\n"
	}

	var parts []string
	for _, e := range entries {
		if e.TryNoDisplay() {
			continue
		}
		if cli.ShowID {
			parts = append(parts, e.AppId)
		}
		if cli.ShowName {
			parts = append(parts, e.Name())
		}
		if cli.ShowPath {
			parts = append(parts, e.Path)
		}
		if len(parts) == 0 {
			continue
		}
		fmt.Print(strings.Join(parts, cli.Delimiter), sep)
		parts = parts[:0]
	}
}
