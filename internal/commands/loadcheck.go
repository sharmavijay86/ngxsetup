package commands

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"ngxsetup/internal/runner"
	"ngxsetup/internal/sysutil"
)

func LoadCheck(args []string) int {
	fsFlags := flag.NewFlagSet("loadcheck", flag.ContinueOnError)
	fsFlags.SetOutput(os.Stdout)
	dryRun := fsFlags.Bool("dry-run", false, "print actions without executing")
	if err := fsFlags.Parse(args); err != nil {
		return 2
	}
	if err := sysutil.MustBeRoot(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	r := runner.Runner{DryRun: *dryRun, Stdout: os.Stdout, Stderr: os.Stderr}
	ctx := context.Background()

	ncpuStr, _ := r.Output(ctx, "nproc")
	ncpu, _ := strconv.Atoi(strings.TrimSpace(ncpuStr))
	if ncpu <= 0 {
		ncpu = 1
	}
	load1, err := readLoad1()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	name := time.Now().Format("02Jan2006")

	if int(load1) > ncpu {
		_ = appendLine("/home/load.log", fmt.Sprintf("%s : current load is %d hence service down", name, int(load1)))
		_ = r.Run(ctx, "service", "nginx", "stop")
	} else {
		fmt.Println("Server Load is ok! ")
	}

	srvStat, _ := r.Output(ctx, "bash", "-lc", "ps -ef | grep -v grep | grep nginx | wc -l")
	n, _ := strconv.Atoi(strings.TrimSpace(srvStat))
	if n == 0 && int(load1) < ncpu {
		_ = r.Run(ctx, "service", "nginx", "start")
	} else {
		fmt.Println("nginx is running!!!")
	}
	return 0
}

func readLoad1() (float64, error) {
	b, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, err
	}
	parts := strings.Fields(string(b))
	if len(parts) < 1 {
		return 0, fmt.Errorf("unexpected /proc/loadavg")
	}
	return strconv.ParseFloat(parts[0], 64)
}

func appendLine(path, line string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	_, _ = w.WriteString(line + "\n")
	return w.Flush()
}
