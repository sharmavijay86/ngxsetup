package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ngxsetup/internal/commands"
)

func Run(args []string) int {
	argv0 := "ngxsetup"
	if len(args) > 0 {
		argv0 = filepath.Base(args[0])
	}

	// Busybox-style: if invoked as vhostsetup/fixperm/etc, run that command.
	switch argv0 {
	case "vhostsetup":
		return commands.VHostSetup(args[1:])
	case "fixperm":
		return commands.FixPerm(args[1:])
	case "loadcheck":
		return commands.LoadCheck(args[1:])
	case "mysqltune":
		return commands.MySQLTune(args[1:])
	case "modsec-install":
		return commands.ModSecInstall(args[1:])
	}

	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, usage())
		return 2
	}

	cmd := strings.ToLower(args[1])
	switch cmd {
	case "setup":
		return commands.Setup(args[2:])
	case "vhostsetup", "vhost":
		return commands.VHostSetup(args[2:])
	case "fixperm":
		return commands.FixPerm(args[2:])
	case "loadcheck":
		return commands.LoadCheck(args[2:])
	case "mysqltune":
		return commands.MySQLTune(args[2:])
	case "modsec-install":
		return commands.ModSecInstall(args[2:])
	case "help", "-h", "--help":
		fmt.Print(usage())
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n%s", cmd, usage())
		return 2
	}
}

func usage() string {
	return `ngxsetup - single-binary Ubuntu Nginx/MySQL/PHP setup

Usage:
  ngxsetup setup [--db=mariadb|mysql] [--dry-run]
  ngxsetup vhostsetup
  ngxsetup fixperm
  ngxsetup loadcheck
  ngxsetup mysqltune
  ngxsetup modsec-install

This binary embeds the original repo assets (common/, conf.d/, nginx/, extra/, docs/)
and reproduces the behavior of the bash scripts.
`
}
