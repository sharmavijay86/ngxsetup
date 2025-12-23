package commands

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	rootassets "ngxsetup"
	"ngxsetup/internal/archive"
	"ngxsetup/internal/assets"
	"ngxsetup/internal/netutil"
	"ngxsetup/internal/runner"
	"ngxsetup/internal/sysutil"
)

func Setup(args []string) int {
	fsFlags := flag.NewFlagSet("setup", flag.ContinueOnError)
	fsFlags.SetOutput(os.Stdout)
	db := fsFlags.String("db", "", "mysql or mariadb")
	dryRun := fsFlags.Bool("dry-run", false, "print commands without executing")
	if err := fsFlags.Parse(args); err != nil {
		return 2
	}

	if err := sysutil.MustBeRoot(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	choice := strings.TrimSpace(strings.ToLower(*db))
	if choice == "" {
		fmt.Print("Enter 'mysql' or press Enter for 'mariadb': ")
		var in string
		fmt.Scanln(&in)
		choice = strings.TrimSpace(strings.ToLower(in))
	}
	if choice != "mysql" {
		choice = "mariadb"
	}

	r := runner.Runner{DryRun: *dryRun, Stdout: os.Stdout, Stderr: os.Stderr}
	ctx := context.Background()

	// 1) Self-signed SSL
	fmt.Println("Generating self-signed SSL cert...")
	if !r.DryRun {
		if err := sysutil.GenerateSelfSigned("/etc/ssl/certs/apache-selfsigned.crt", "/etc/ssl/private/apache-selfsigned.key"); err != nil {
			fmt.Fprintln(os.Stderr, "self-signed cert:", err)
			return 1
		}
	}

	// 2) Enable remote access: append embedded key
	fmt.Println("Configuring /root/.ssh/authorized_keys...")
	key, err := fs.ReadFile(rootassets.Embedded, "extra/key")
	if err != nil {
		fmt.Fprintln(os.Stderr, "read embedded key:", err)
		return 1
	}
	if err := sysutil.EnsureDir("/root/.ssh", 0o700); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if !r.DryRun {
		if err := sysutil.AppendFile("/root/.ssh/authorized_keys", key, 0o600); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}

	// 3) apt update
	if err := r.Run(ctx, "apt-get", "update"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	// 4) Database
	if choice == "mysql" {
		fmt.Println("Installing MySQL...")
		if err := r.Run(ctx, "apt", "install", "mysql-server", "-y"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	} else {
		fmt.Println("Installing MariaDB (default)...")
		if err := r.Run(ctx, "apt", "install", "mariadb-server", "-y"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	fmt.Println("Database installation complete!")

	// 5) Base packages
	if err := r.Run(ctx, "apt-get", "install", "-yq", "nginx-extras", "net-tools", "python3-certbot-nginx", "qemu-guest-agent"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := r.Run(ctx, "apt-get", "install", "-yq",
		"build-essential", "ca-certificates", "wget", "curl", "libpcre3", "libpcre3-dev", "autoconf", "unzip", "automake", "libtool", "tar", "git", "libssl-dev", "zlib1g-dev", "uuid-dev", "lsb-release", "vim", "htop", "sysstat", "ufw", "fail2ban", "makepasswd"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	// 6) Copy nginx configs from embedded assets
	fmt.Println("Installing nginx configuration assets...")
	if !r.DryRun {
		// common/ -> /etc/nginx/common
		if err := assets.ExtractDir(rootassets.Embedded, "common", "/etc/nginx/common", nil); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		// conf.d/ -> /etc/nginx/conf.d
		if err := assets.ExtractDir(rootassets.Embedded, "conf.d", "/etc/nginx/conf.d", nil); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		// nginx/nginx.conf -> /etc/nginx/nginx.conf
		nginxConf, err := fs.ReadFile(rootassets.Embedded, "nginx/nginx.conf")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if err := sysutil.WriteFileAtomic("/etc/nginx/nginx.conf", nginxConf, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}

		// nginx/def* -> /etc/nginx/sites-available/
		// Walk embedded nginx/ and install only def* files.
		err = fs.WalkDir(rootassets.Embedded, "nginx", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			base := filepath.Base(path)
			if !strings.HasPrefix(base, "def") {
				return nil
			}
			b, err := fs.ReadFile(rootassets.Embedded, path)
			if err != nil {
				return err
			}
			target := filepath.Join("/etc/nginx/sites-available", base)
			return sysutil.WriteFileAtomic(target, b, 0o644)
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}

	// 7) PHP install + tuning
	if err := r.Run(ctx, "apt-get", "install", "-yq", "php", "php-fpm", "php-mysql", "php-gd", "php-curl", "php-cgi", "php-cli", "php-json", "php-memcached", "php-mbstring", "php-xml", "memcached"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	pver, err := r.Output(ctx, "php", "-r", "echo PHP_MAJOR_VERSION.'.'.PHP_MINOR_VERSION;")
	if err != nil {
		fmt.Fprintln(os.Stderr, "detect php version:", err)
		return 1
	}

	if !r.DryRun {
		// Replace 7.4 placeholder in def* vhost templates.
		entries, _ := os.ReadDir("/etc/nginx/sites-available")
		for _, e := range entries {
			if e.IsDir() || !strings.HasPrefix(e.Name(), "def") {
				continue
			}
			path := filepath.Join("/etc/nginx/sites-available", e.Name())
			b, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			updated := strings.ReplaceAll(string(b), "7.4", pver)
			_ = sysutil.WriteFileAtomic(path, []byte(updated), 0o644)
		}

		phpIni := filepath.Join("/etc/php", pver, "fpm/php.ini")
		if err := updateIniLine(phpIni, map[string]string{
			"memory_limit":        "1024M",
			"upload_max_filesize": "512M",
			"post_max_size":       "512M",
			"max_execution_time":  "18000",
		}); err != nil {
			fmt.Fprintln(os.Stderr, "php.ini:", err)
			return 1
		}
		wwwConf := filepath.Join("/etc/php", pver, "fpm/pool.d/www.conf")
		if err := tunePhpFpmPool(wwwConf, r); err != nil {
			fmt.Fprintln(os.Stderr, "php-fpm pool:", err)
			return 1
		}
	}

	// 8) phpMyAdmin install
	fmt.Println("Installing phpMyAdmin...")
	if !r.DryRun {
		zipPath := "/tmp/phpmyadmin.zip"
		if err := netutil.DownloadToFile("https://files.phpmyadmin.net/phpMyAdmin/5.2.3/phpMyAdmin-5.2.3-english.zip", zipPath, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		tmpDir := "/tmp/phpmyadmin_extract"
		_ = os.RemoveAll(tmpDir)
		if err := archive.Unzip(zipPath, tmpDir); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		src := filepath.Join(tmpDir, "phpMyAdmin-5.2.3-english")
		_ = os.RemoveAll("/usr/share/phpmyadmin")
		if err := os.Rename(src, "/usr/share/phpmyadmin"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		_ = os.MkdirAll("/var/www/html", 0o755)
		if err := chownR("/usr/share/phpmyadmin", "www-data", "www-data", r); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		_ = os.Remove(zipPath)
		_ = os.RemoveAll(tmpDir)
	}

	// 9) sysstat
	if !r.DryRun {
		_ = replaceInFile("/etc/default/sysstat", "ENABLED=\"false\"", "ENABLED=\"true\"")
	}
	_ = r.Run(ctx, "systemctl", "enable", "sysstat")
	_ = r.Run(ctx, "systemctl", "restart", "sysstat")

	// 10) Cloudflare real IP config
	fmt.Println("Writing Cloudflare real_ip config...")
	if !r.DryRun {
		cfPath := "/etc/nginx/conf.d/cf.conf"
		_ = os.Remove(cfPath)
		_ = sysutil.AppendFile(cfPath, []byte("real_ip_header CF-Connecting-IP;\n"), 0o644)
		v4, _ := netutil.GetText("https://www.cloudflare.com/ips-v4")
		for _, line := range strings.Split(v4, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			_ = sysutil.AppendFile(cfPath, []byte("set_real_ip_from "+line+";\n"), 0o644)
		}
		v6, _ := netutil.GetText("https://www.cloudflare.com/ips-v6")
		for _, line := range strings.Split(v6, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			_ = sysutil.AppendFile(cfPath, []byte("set_real_ip_from "+line+";\n"), 0o644)
		}
	}

	// 11) sysctl
	fmt.Println("Applying sysctl tweaks...")
	sysctlTxt, err := fs.ReadFile(rootassets.Embedded, "extra/sysctl.txt")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if !r.DryRun {
		_ = sysutil.AppendFile("/etc/sysctl.conf", sysctlTxt, 0o644)
	}
	_ = r.Run(ctx, "sysctl", "-p")

	// 12) Install symlinks for single-binary tools + generate /root/.pw
	fmt.Println("Installing helper commands (vhostsetup, fixperm, etc.)...")
	if err := installSelfAndLinks(r); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if !r.DryRun {
		pw, _ := sysutil.RandomStringAlphaNum(13)
		_ = sysutil.WriteFileAtomic("/root/.pw", []byte(pw), 0o600)
	}

	// 13) fail2ban
	fmt.Println("Configuring fail2ban...")
	if err := r.Run(ctx, "cp", "-f", "/etc/fail2ban/jail.conf", "/etc/fail2ban/jail.local"); err != nil {
		// fail2ban may not be installed/available yet; keep behavior permissive.
		fmt.Fprintln(os.Stderr, "warn:", err)
	}
	jailTxt, _ := fs.ReadFile(rootassets.Embedded, "extra/jail.txt")
	if !r.DryRun {
		_ = sysutil.AppendFile("/etc/fail2ban/jail.local", jailTxt, 0o644)
		xmlrpc, _ := fs.ReadFile(rootassets.Embedded, "extra/xmlrpc.conf")
		_ = os.MkdirAll("/etc/fail2ban/filter.d", 0o755)
		_ = sysutil.WriteFileAtomic("/etc/fail2ban/filter.d/xmlrpc.conf", xmlrpc, 0o644)
		motd, _ := fs.ReadFile(rootassets.Embedded, "extra/50-cti")
		_ = sysutil.WriteFileAtomic("/etc/update-motd.d/50-cti", motd, 0o755)
	}

	// 14) Remove apache2
	_ = r.Run(ctx, "apt-get", "remove", "apache2", "-y")

	// 15) Clear history (approximation of `history -c`)
	if !r.DryRun {
		_ = os.Remove("/root/.bash_history")
	}

	// 16) wp-cli install
	fmt.Println("Installing WP-CLI...")
	if !r.DryRun {
		phar := "/tmp/wp-cli.phar"
		if err := netutil.DownloadToFile("https://raw.githubusercontent.com/wp-cli/builds/gh-pages/phar/wp-cli.phar", phar, 0o755); err == nil {
			_ = os.Rename(phar, "/usr/local/bin/wp")
		}
	}
	_ = r.Run(ctx, "apt", "auto-remove", "-y")

	fmt.Println("Installation done.")
	return 0
}

func replaceInFile(path, old, new string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	out := strings.ReplaceAll(string(b), old, new)
	return sysutil.WriteFileAtomic(path, []byte(out), 0o644)
}

func updateIniLine(path string, keyToValue map[string]string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(b), "\n")
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		for key, val := range keyToValue {
			if strings.HasPrefix(trim, key+" ") || strings.HasPrefix(trim, key+"=") {
				lines[i] = key + " = " + val
			}
		}
	}
	return sysutil.WriteFileAtomic(path, []byte(strings.Join(lines, "\n")), 0o644)
}

func tunePhpFpmPool(path string, r runner.Runner) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	cpu, _ := r.Output(context.Background(), "nproc")
	if cpu == "" {
		cpu = "1"
	}
	lines := strings.Split(string(b), "\n")
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "pm =") {
			lines[i] = "pm = ondemand"
			continue
		}
		if strings.HasPrefix(trim, "pm.max_children") {
			lines[i] = "pm.max_children = " + cpu
			continue
		}
		if strings.Contains(trim, "start_servers") || strings.Contains(trim, "min_spare_servers") || strings.Contains(trim, "max_spare_servers") {
			if !strings.HasPrefix(strings.TrimSpace(line), ";") {
				lines[i] = ";" + line
			}
			continue
		}
		if strings.Contains(trim, "pm.process_idle_timeout") {
			lines[i] = "pm.process_idle_timeout = 10s"
			continue
		}
		if strings.HasPrefix(trim, ";pm.max_requests") || strings.HasPrefix(trim, "pm.max_requests") {
			lines[i] = "pm.max_requests = 5000"
			continue
		}
	}
	return sysutil.WriteFileAtomic(path, []byte(strings.Join(lines, "\n")), 0o644)
}

func installSelfAndLinks(r runner.Runner) error {
	if r.DryRun {
		fmt.Println("+ install self to /usr/local/bin/ngxsetup and symlinks (vhostsetup, fixperm, loadcheck, mysqltune, modsec-install)")
		return nil
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(self)
	if err != nil {
		return err
	}
	if err := sysutil.WriteFileAtomic("/usr/local/bin/ngxsetup", data, 0o755); err != nil {
		return err
	}
	links := []string{"vhostsetup", "fixperm", "loadcheck", "mysqltune", "modsec-install"}
	for _, name := range links {
		p := filepath.Join("/usr/local/bin", name)
		_ = os.Remove(p)
		if err := os.Symlink("/usr/local/bin/ngxsetup", p); err != nil {
			return err
		}
	}
	return nil
}

func chownR(path, user, group string, r runner.Runner) error {
	uidStr, err := r.Output(context.Background(), "id", "-u", user)
	if err != nil {
		return err
	}
	gidStr, err := r.Output(context.Background(), "id", "-g", group)
	if err != nil {
		return err
	}
	uid := atoi(uidStr)
	gid := atoi(gidStr)
	return filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Chown(p, uid, gid)
	})
}

func atoi(s string) int {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}
