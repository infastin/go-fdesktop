package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"

	"github.com/adrg/xdg"
	"github.com/alecthomas/kong"
	"github.com/infastin/fdesktop-go"
)

var cli struct {
	ShowID        bool   `optional:"" name:"id" short:"i" help:"Print AppID."`
	ShowPath      bool   `optional:"" name:"path" short:"p" default:"true" negatable:"" help:"Print Path."`
	ShowName      bool   `optional:"" name:"name" short:"n" default:"true" negatable:"" help:"Print Name."`
	Delimiter     string `optional:"" name:"delimiter" short:"d" default:"\t" help:"Delimiter for printed attributes."`
	NullDelimiter bool   `optional:"" name:"null-delimiter" short:"z" help:"Use the null character as a delimiter for printed attributes."`
	Null          bool   `optional:"" name:"null" short:"0" help:"Separate results by the null character."`
	JSON          bool   `optional:"" name:"json" short:"j" help:"Output as JSON array."`
}

func parseEntries() (entries []*fdesktop.Entry, err error) {
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
			return nil, err
		}
	}
	return entries, nil
}

func printEntriesPlain(entries []*fdesktop.Entry) {
	var sep string
	if cli.Null {
		sep = "\000"
	} else {
		sep = "\n"
	}

	var delim string
	if cli.NullDelimiter {
		delim = "\000"
	} else {
		delim = cli.Delimiter
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
		fmt.Print(strings.Join(parts, delim), sep)
		parts = parts[:0]
	}
}

func printEntriesJSON(entries []*fdesktop.Entry) {
	type jsonEntry struct {
		AppID string
		Name  string
		Path  string
	}

	jsonEntries := make([]jsonEntry, 0, len(entries))
	for _, entry := range entries {
		jsonEntries = append(jsonEntries, jsonEntry{
			AppID: entry.AppId,
			Name:  entry.Name(),
			Path:  entry.Path,
		})
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")

	encoder.Encode(jsonEntries)
}

func main() {
	kong.Parse(&cli)

	entries, err := parseEntries()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse desktop entries: %s", err.Error())
		os.Exit(1)
	}

	if !cli.JSON {
		printEntriesPlain(entries)
	} else {
		printEntriesJSON(entries)
	}
}
