package commands

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"ngxsetup/internal/sysutil"
)

func FixPerm(args []string) int {
	fsFlags := flag.NewFlagSet("fixperm", flag.ContinueOnError)
	fsFlags.SetOutput(os.Stdout)
	if err := fsFlags.Parse(args); err != nil {
		return 2
	}
	if err := sysutil.MustBeRoot(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	entries, err := os.ReadDir("/var/www")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		wpContent := filepath.Join("/var/www", e.Name(), "wp-content")
		if _, err := os.Stat(wpContent); err != nil {
			continue
		}
		_ = os.Chown(wpContent, 33, 33) // default www-data uid/gid on Ubuntu/Debian
		_ = filepath.Walk(wpContent, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			_ = os.Chown(p, 33, 33)
			return nil
		})
	}
	return 0
}
