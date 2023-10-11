package main

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"strings"

	"github.com/adrg/xdg"
	"gitlab.com/infastin/go-fdesktop"
	"gitlab.com/infastin/go-flag/v2"
)

func main() {
	var showName bool
	var showPath bool
	var showId bool
	var delim string
	var help bool
	var null bool

	flag.Var(&showId, "id", 'i', false, "If specified AppID will be printed")
	flag.Var(&showPath, "path", 'p', false, "If specified Path will be printed")
	flag.Var(&showName, "name", 'n', false, "If specified Name will be printed")
	flag.Var(&delim, "delim", 'd', ":", "Delimiter for shown attributes")
	flag.Var(&help, "help", 'h', false, "Print help message")
	flag.Var(&null, "null", '0', false, "Separate results by the null byte")

	flag.Parse()

	if help || (!showName && !showPath && !showId) {
		flag.PrintUsage(os.Stdout)
		return
	}

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

		if showId {
			b.WriteString(e.AppId)
		}

		if showId && (showName || showPath) {
			b.WriteString(delim)
		}

		if showName {
			b.WriteString(e.Name())
		}

		if showName && showPath {
			b.WriteString(delim)
		}

		if showPath {
			b.WriteString(e.Path)
		}

		if null {
			b.WriteRune(0)
		} else {
			b.WriteRune('\n')
		}

		fmt.Print(b.String())
		b.Reset()
	}
}
