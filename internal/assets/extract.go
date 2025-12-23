package assets

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func ExtractDir(embedded fs.FS, srcDir, dstDir string, permOverride *os.FileMode) error {
	return fs.WalkDir(embedded, srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(path, srcDir)
		rel = strings.TrimPrefix(rel, string(filepath.Separator))
		target := filepath.Join(dstDir, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		srcFile, err := embedded.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()
		data, err := io.ReadAll(srcFile)
		if err != nil {
			return err
		}
		perm := os.FileMode(0o644)
		if permOverride != nil {
			perm = *permOverride
		}
		return os.WriteFile(target, data, perm)
	})
}
