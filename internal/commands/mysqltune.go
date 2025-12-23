package commands

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"ngxsetup/internal/netutil"
	"ngxsetup/internal/runner"
	"ngxsetup/internal/sysutil"
)

func MySQLTune(args []string) int {
	fsFlags := flag.NewFlagSet("mysqltune", flag.ContinueOnError)
	fsFlags.SetOutput(os.Stdout)
	dryRun := fsFlags.Bool("dry-run", false, "print actions without executing")
	if err := fsFlags.Parse(args); err != nil {
		return 2
	}
	if err := sysutil.MustBeRoot(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	ctx := context.Background()
	r := runner.Runner{DryRun: *dryRun, Stdout: os.Stdout, Stderr: os.Stderr}

	totalMemGB, err := memGB(r)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	totalCPU, _ := strconv.Atoi(strings.TrimSpace(must(r.Output(ctx, "nproc"))))
	if totalCPU <= 0 {
		totalCPU = 1
	}
	date := time.Now().Format("20060102_150405")

	bufferPoolSize := int(float64(totalMemGB) * 0.75)
	if bufferPoolSize < 1 {
		bufferPoolSize = 1
	}
	maxConnections := totalMemGB * 100
	threadCache := totalCPU * 12

	getVal := func(param, def string) string {
		b, err := os.ReadFile("/etc/mysql/mysql.conf.d/mysqld.cnf")
		if err != nil {
			return def
		}
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, param) {
				fields := strings.Fields(line)
				if len(fields) > 0 {
					return fields[len(fields)-1]
				}
			}
		}
		return def
	}

	mysqlUser := getVal("user", "mysql")
	pidFile := getVal("pid-file", "/var/run/mysqld/mysqld.pid")
	socket := getVal("socket", "/var/run/mysqld/mysqld.sock")
	port := getVal("port", "3306")
	datadir := getVal("datadir", "/var/lib/mysql")
	bind := getVal("bind-address", "0.0.0.0")

	fmt.Println("Backing up current MySQL configuration...")
	backup := "/etc/mysql/mysql.conf.d/mysqld.cnf.backup." + date
	if err := r.Run(ctx, "cp", "/etc/mysql/mysql.conf.d/mysqld.cnf", backup); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	newCfg := fmt.Sprintf(`[mysqld]
# Basic Settings
user                    = %s
pid-file                = %s
socket                  = %s
port                    = %s
datadir                 = %s
bind-address            = %s

# Buffer Pool Settings
innodb_buffer_pool_size = %dG
innodb_buffer_pool_instances = %d

# InnoDB Settings
innodb_file_per_table  = 1
innodb_flush_method    = O_DIRECT
innodb_log_file_size   = 2G
innodb_log_buffer_size = 128M
innodb_write_io_threads = %d
innodb_read_io_threads  = %d
innodb_io_capacity     = 2000
innodb_io_capacity_max = 4000
innodb_flush_log_at_trx_commit = 2
innodb_lock_wait_timeout = 120
innodb_ft_min_token_size = 2
innodb_ft_enable_stopword = 0

# Connection Settings
max_connections        = %d
thread_cache_size     = %d
table_open_cache      = 8000
table_open_cache_instances = %d
thread_stack          = 256K
max_allowed_packet    = 128M

# Temporary Tables
tmp_table_size        = 4G
max_heap_table_size   = 4G

# Search and Sort Settings
sort_buffer_size      = 8M
read_buffer_size      = 2M
read_rnd_buffer_size  = 2M
join_buffer_size      = 2M

# Binary Log Settings
server_id             = 1
log_bin               = mysql-bin
expire_logs_days      = 7
binlog_format         = ROW
sync_binlog          = 1

# Performance Schema
performance_schema = ON
performance_schema_max_table_instances = 1000
performance_schema_max_table_handles = 1000

# Character Set
character-set-server  = utf8mb4
collation-server      = utf8mb4_0900_ai_ci
default_authentication_plugin = mysql_native_password

# MySQL 8.0 Specific Settings
innodb_dedicated_server = ON
innodb_buffer_pool_load_at_startup = 1
innodb_buffer_pool_dump_at_shutdown = 1
`, mysqlUser, pidFile, socket, port, datadir, bind, bufferPoolSize, totalCPU, totalCPU, totalCPU, maxConnections, threadCache, totalCPU)

	if !r.DryRun {
		if err := sysutil.WriteFileAtomic("/etc/mysql/mysql.conf.d/mysqld.cnf", []byte(newCfg), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		sysctlCfg := `vm.swappiness = 1
vm.dirty_background_ratio = 2
vm.dirty_ratio = 40
net.core.somaxconn = 65535
net.ipv4.tcp_max_syn_backlog = 65535
fs.aio-max-nr = 1048576
fs.file-max = 6815744
`
		_ = sysutil.WriteFileAtomic("/etc/sysctl.d/99-mysql.conf", []byte(sysctlCfg), 0o644)
	}
	_ = r.Run(ctx, "sysctl", "-p", "/etc/sysctl.d/99-mysql.conf")

	_ = r.Run(ctx, "apt-get", "update")
	_ = r.Run(ctx, "apt-get", "install", "-y", "percona-toolkit", "mytop")

	if !r.DryRun {
		_ = netutil.DownloadToFile("https://raw.githubusercontent.com/major/MySQLTuner-perl/master/mysqltuner.pl", "/usr/local/bin/mysqltuner.pl", 0o755)
	}

	fmt.Println("Restarting MySQL...")
	_ = r.Run(ctx, "systemctl", "restart", "mysql")
	fmt.Println("Waiting for MySQL to start...")
	if !r.DryRun {
		time.Sleep(10 * time.Second)
	}
	if err := r.Run(ctx, "systemctl", "is-active", "--quiet", "mysql"); err == nil {
		fmt.Println("MySQL successfully restarted")
		fmt.Println("Configuration complete! New config saved at /etc/mysql/mysql.conf.d/mysqld.cnf")
		fmt.Println("Backup saved at", backup)
		fmt.Println("Run 'mysqltuner.pl' after 48 hours for performance analysis")
		return 0
	}

	fmt.Println("Error: MySQL failed to start. Rolling back changes...")
	_ = r.Run(ctx, "cp", backup, "/etc/mysql/mysql.conf.d/mysqld.cnf")
	_ = r.Run(ctx, "systemctl", "restart", "mysql")
	return 1
}

func memGB(r runner.Runner) (int, error) {
	out, err := r.Output(context.Background(), "bash", "-lc", "free -g | grep Mem | awk '{print $2}'")
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0, err
	}
	if n < 1 {
		n = 1
	}
	return n, nil
}

func must(s string, err error) string {
	if err != nil {
		return ""
	}
	return s
}
