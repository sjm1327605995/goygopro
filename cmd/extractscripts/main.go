// mkcdb extracts scripts.tgz (a GitHub tarball of ygopro-scripts) into
// script/. One-off helper used to vendor the card scripts without tar.
package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func run() error {
	// scripts.tgz → script/ (the whole ygopro-scripts repo).
	n, err := extract("scripts.tgz", func(rel string) (string, bool) {
		if rel == "" {
			return "", false
		}
		return filepath.Join("script", filepath.FromSlash(rel)), true
	})
	if err != nil {
		return err
	}
	fmt.Printf("extracted %d files into script/\n", n)

	// ygopro-database tarball → cards-<locale>.cdb at the repo root.
	dbArchive := "database.tgz"
	if _, err := os.Stat(dbArchive); err != nil {
		dbArchive = "cards_probe1.cdb" // misnamed probe download of the same tarball
	}
	m, err := extract(dbArchive, func(rel string) (string, bool) {
		base := filepath.Base(filepath.FromSlash(rel))
		if base != "cards.cdb" {
			return "", false
		}
		locale := filepath.Base(filepath.Dir(filepath.FromSlash(rel)))
		if locale == "." || locale == "/" {
			locale = "en"
		}
		return fmt.Sprintf("cards-%s.cdb", locale), true
	})
	if err != nil {
		return err
	}
	fmt.Printf("extracted %d database files\n", m)
	return nil
}

// extract pulls entries out of a .tgz archive; the rel path is the entry name
// minus the leading repo directory, and mapTo decides the destination (or skip).
func extract(archive string, mapTo func(rel string) (string, bool)) (int, error) {
	f, err := os.Open(archive)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return 0, err
	}
	tr := tar.NewReader(gz)

	count := 0
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return count, err
		}
		// Strip the leading repo dir (e.g. ygopro-scripts-master/).
		rel := strings.SplitN(hdr.Name, "/", 2)
		var relPath string
		if len(rel) == 2 {
			relPath = rel[1]
		}
		target, ok := mapTo(relPath)
		if !ok {
			continue
		}
		if hdr.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(target, 0755); err != nil {
				return count, err
			}
			continue
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return count, err
		}
		out, err := os.Create(target)
		if err != nil {
			return count, err
		}
		if _, err := io.Copy(out, tr); err != nil {
			out.Close()
			return count, err
		}
		out.Close()
		count++
	}
	return count, nil
}

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}
