// Package source finds the files of a project on disk.
package source

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

// Files returns the files under dir with the given extension, sorted,
// skipping hidden folders.
func Files(dir, ext string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() && path != dir && strings.HasPrefix(e.Name(), ".") {
			return filepath.SkipDir
		}
		if !e.IsDir() && strings.EqualFold(filepath.Ext(path), ext) {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}
