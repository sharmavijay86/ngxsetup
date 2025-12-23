package commands

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ngxsetup/internal/archive"
	"ngxsetup/internal/netutil"
	"ngxsetup/internal/runner"
	"ngxsetup/internal/sysutil"
)

func VHostSetup(args []string) int {
	fsFlags := flag.NewFlagSet("vhostsetup", flag.ContinueOnError)
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

	fmt.Print("############  Script to setup nginx vhost #############\n")
	if _, err := os.Stat("/etc/nginx/sites-available/default"); err != nil {
		fmt.Println("config file does not exist contact helpdesk!")
		return 0
	}
	if _, err := os.Stat("/etc/nginx/sites-available/defaultssl"); err != nil {
		fmt.Println("config file does not exist contact helpdesk!")
		return 0
	}

	for {
		fmt.Print(" Press v if you want to setup a new domain only \n")
		fmt.Print(" Press vs if you want to setup a new SSL domain only \n")
		fmt.Print(" Press w if you want to setup a new domain with wordpress \n")
		fmt.Print(" Press ws if you want to setup a new SSL domain with wordpress \n")
		fmt.Print(" Press x to exit \n")
		fmt.Print("Type your choice:")

		var option string
		fmt.Scanln(&option)
		switch strings.ToLower(strings.TrimSpace(option)) {
		case "v":
			return vhostDomainOnly(ctx, r, false)
		case "vs":
			return vhostDomainOnly(ctx, r, true)
		case "w":
			return vhostWordPress(ctx, r, false)
		case "ws":
			return vhostWordPress(ctx, r, true)
		case "x":
			return 0
		default:
			fmt.Println("Wrong choice! Please select correct option menu!")
		}
	}
}

func vhostDomainOnly(ctx context.Context, r runner.Runner, ssl bool) int {
	dn := prompt("Give your domain name:")
	fname := strings.ReplaceAll(dn, ".", "")
	rootInfo := "/root/" + fname
	_ = sysutil.AppendFile(rootInfo, []byte("your domain name is: "+dn+"\n"), 0o644)
	_ = sysutil.AppendFile(rootInfo, []byte("your document root path is: /var/www/"+fname+"\n"), 0o644)

	template := "/etc/nginx/sites-available/default"
	if ssl {
		template = "/etc/nginx/sites-available/defaultssl"
	}
	out := "/etc/nginx/sites-enabled/" + fname
	if err := renderVhost(template, out, dn, fname, r); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := os.MkdirAll("/var/www/"+fname, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	_ = r.Run(ctx, "chown", "-R", "www-data:www-data", "/var/www/"+fname)

	fmt.Println("######################################################")
	fmt.Printf("Domain has been setup put your files here at /var/www/%s.\n", fname)
	fmt.Printf("Dont forget to run fixperm command to fix owenership.\n")
	fmt.Printf(" Need to setup another domain? please run vhostsetup command again! \n")
	fmt.Println("######################################################")
	return 0
}

func vhostWordPress(ctx context.Context, r runner.Runner, ssl bool) int {
	dn := prompt("Give your domain name:")
	fname := strings.ReplaceAll(dn, ".", "")
	rootInfo := "/root/" + fname
	_ = sysutil.AppendFile(rootInfo, []byte("your domain name is: "+dn+"\n"), 0o644)
	_ = sysutil.AppendFile(rootInfo, []byte("your document root path is: /var/www/"+fname+"\n"), 0o644)

	dbr := fname
	if len(dbr) > 6 {
		dbr = dbr[:6]
	}
	hex, _ := sysutil.RandomHex(3)
	dbn := dbr + hex
	pass, _ := sysutil.RandomStringAlphaNum(13)

	_ = sysutil.AppendFile(rootInfo, []byte("Your database name is: "+dbn+"\n"), 0o644)
	_ = sysutil.AppendFile(rootInfo, []byte("Your database user is: "+dbn+"\n"), 0o644)
	_ = sysutil.AppendFile(rootInfo, []byte("Your database user password is: "+pass+"\n"), 0o644)
	if ssl {
		_ = sysutil.AppendFile(rootInfo, []byte("Your website login id is: wpadmin \n"), 0o644)
		_ = sysutil.AppendFile(rootInfo, []byte("Your website wpadmin user password is: "+pass+"\n"), 0o644)
	}

	template := "/etc/nginx/sites-available/default"
	if ssl {
		template = "/etc/nginx/sites-available/defaultssl"
	}
	out := "/etc/nginx/sites-enabled/" + fname
	if err := renderVhost(template, out, dn, fname, r); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	if err := ensureDB(ctx, r, dbn, pass); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	fmt.Println("Downloading WordPress...")
	if !r.DryRun {
		zipPath := "/tmp/wordpress.zip"
		if err := netutil.DownloadToFile("https://wordpress.org/latest.zip", zipPath, 0o644); err != nil {
			return fail(err)
		}
		tmpDir := "/tmp/wordpress_extract"
		_ = os.RemoveAll(tmpDir)
		if err := archive.Unzip(zipPath, tmpDir); err != nil {
			return fail(err)
		}
		_ = os.RemoveAll("/var/www/" + fname)
		if err := os.Rename(filepath.Join(tmpDir, "wordpress"), "/var/www/"+fname); err != nil {
			return fail(err)
		}
		_ = os.Remove(zipPath)
		_ = os.RemoveAll(tmpDir)
	}
	_ = r.Run(ctx, "chown", "-R", "www-data:www-data", "/var/www/"+fname)

	if ssl {
		// Mimic the bash behavior: pre-configure WP + certbot.
		_ = r.Run(ctx, "sudo", "-u", "www-data", "wp", "config", "create", "--dbname="+dbn, "--dbuser="+dbn, "--dbpass="+pass, "--path=/var/www/"+fname)
		_ = r.Run(ctx, "sudo", "-u", "www-data", "wp", "core", "install", "--url="+dn, "--title=Crazy Tech India sample", "--admin_user=wpadmin", "--admin_password="+pass, "--admin_email=vijay@mevijay.dev", "--skip-email", "--path=/var/www/"+fname)
		_ = r.Run(ctx, "nginx", "-t")
		_ = r.Run(ctx, "systemctl", "restart", "nginx")
		_ = r.Run(ctx, "certbot", "certonly", "--nginx", "-d", dn, "-d", "www."+dn, "--agree-tos", "-m", "vijay@mevijay.dev", "-n")
		fmt.Println("######################################################")
		fmt.Printf("Open url %s in your browser, site is ready. \n read file /root/%s for database and wp information \n", dn, fname)
		fmt.Printf(" Need to setup another domain? please run vhostsetup command again! \n")
		fmt.Println("######################################################")
		return 0
	}

	fmt.Println("######################################################")
	fmt.Printf("Open url %s in your browser and install wordpress. \n read file /root/%s for database information to be use during installation\n", dn, fname)
	fmt.Printf(" Need to setup another domain? please run vhostsetup command again! \n")
	fmt.Println("######################################################")
	return 0
}

func ensureDB(ctx context.Context, r runner.Runner, dbn, pass string) error {
	// Try without password first (matches extra/vhostsetup), then fallback to /root/.pw if needed.
	try := func(args ...string) error {
		return r.Run(ctx, "mysql", args...)
	}

	cmds := []string{
		"CREATE DATABASE " + dbn,
		"CREATE USER '" + dbn + "'@'localhost' IDENTIFIED BY'" + pass + "';",
		"GRANT ALL PRIVILEGES ON " + dbn + ".* to '" + dbn + "'@'localhost';",
		"FLUSH PRIVILEGES;",
	}
	for _, q := range cmds {
		if err := try("-e", q); err != nil {
			pw, pwErr := os.ReadFile("/root/.pw")
			if pwErr != nil {
				return err
			}
			pwStr := strings.TrimSpace(string(pw))
			return r.Run(ctx, "mysql", "-u", "root", "-p"+pwStr, "-e", q)
		}
	}
	return nil
}

func renderVhost(templatePath, outPath, domain, fname string, r runner.Runner) error {
	b, err := os.ReadFile(templatePath)
	if err != nil {
		return err
	}
	content := string(b)
	content = strings.ReplaceAll(content, "localhost", domain+" www."+domain)
	content = strings.ReplaceAll(content, "/www/html", "/www/"+fname)
	if r.DryRun {
		fmt.Printf("+ write %s (from %s)\n", outPath, templatePath)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	return sysutil.WriteFileAtomic(outPath, []byte(content), 0o644)
}

func prompt(msg string) string {
	fmt.Print(msg)
	var s string
	fmt.Scanln(&s)
	return strings.TrimSpace(s)
}

func fail(err error) int {
	fmt.Fprintln(os.Stderr, err)
	return 1
}
