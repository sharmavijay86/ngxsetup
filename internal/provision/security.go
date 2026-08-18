package provision

import (
	"fmt"
	"os"
	"strings"

	"ngxsetup/internal/logx"
	"ngxsetup/internal/system"
	"ngxsetup/internal/tmpl"
)

// ApplySecurity configures fail2ban, the firewall and automatic security
// updates.
//
// Notably absent: this tool does not install an SSH key. The previous
// implementation embedded one in the binary and appended it to
// /root/.ssh/authorized_keys on every machine it provisioned, which handed
// permanent root on every one of those servers to whoever held the matching
// private key. Access is the operator's to grant.
func (c *Ctx) ApplySecurity() error {
	if err := c.applyFail2ban(); err != nil {
		return err
	}
	if err := c.applyFirewall(); err != nil {
		return err
	}
	c.applyUnattendedUpgrades()
	c.reportSSHConfiguration()
	return nil
}

func (c *Ctx) applyFail2ban() error {
	if !c.Runner.Look("fail2ban-server") {
		logx.Warn("fail2ban is not installed; the login and scanner jails will not run")
		return nil
	}

	filters := map[string]string{
		"ngxsetup-wp-login": "fail2ban/filter-wp-login.conf",
		"ngxsetup-xmlrpc":   "fail2ban/filter-xmlrpc.conf",
		"ngxsetup-scanner":  "fail2ban/filter-scanner.conf",
	}
	for name, src := range filters {
		body, err := tmpl.RenderRaw(src)
		if err != nil {
			return err
		}
		if _, err := c.Writer.Write("/etc/fail2ban/filter.d/"+name+".conf", body, 0o644, false); err != nil {
			return err
		}
	}

	ignore := append([]string{"127.0.0.1/8", "::1"}, c.Config.TrustedNetworks...)
	jail, err := tmpl.Render("fail2ban/jail.local.tmpl", tmpl.Fail2ban{
		// "systemd" here was wrong: it makes every jail read the journal via
		// journalmatch and ignore `logpath` entirely, including the nginx
		// access-log jails (ngxsetup-wp-login, ngxsetup-xmlrpc,
		// ngxsetup-scanner) and even nginx-http-auth/nginx-limit-req, which
		// read nginx's *file* logs — nginx does not log to the journal, so
		// those jails could never match a single line no matter what a
		// client sent. Confirmed live: 8 deliberately bad wp-login attempts
		// produced zero "Currently failed" on ngxsetup-wp-login, and
		// `fail2ban-client get ngxsetup-wp-login logpath` reported "No file
		// is currently monitored". "auto" is fail2ban's own stock default
		// (jail.conf ships `backend = auto`, not systemd) and watches the
		// files each jail's `logpath` actually names. sshd is kept on
		// systemd explicitly below since Ubuntu's sshd already logs there.
		Backend:   "auto",
		IgnoreIPs: strings.Join(ignore, " "),
	})
	if err != nil {
		return err
	}
	if _, err := c.Writer.Write("/etc/fail2ban/jail.d/ngxsetup.local", jail, 0o644, false); err != nil {
		return err
	}

	if c.Writer.DryRun {
		return nil
	}
	if err := system.Restart(c.Context, c.Runner, "fail2ban.service"); err != nil {
		// A bad jail definition must not be left in place silently.
		logx.Warn("fail2ban did not restart: %v", err)
		logx.Warn("%s", system.JournalTail(c.Context, c.Runner, "fail2ban.service", 15))
	}
	return nil
}

// applyFirewall configures ufw.
//
// The SSH rule goes in before the default-deny policy, in that order. Reversing
// them locks the operator out of the machine they are provisioning, which is
// the classic way to lose a server to its own hardening script.
func (c *Ctx) applyFirewall() error {
	if !c.Runner.Look("ufw") {
		logx.Warn("ufw is not installed; no firewall rules were applied")
		return nil
	}
	if c.Writer.DryRun {
		logx.Change("[dry-run] would allow SSH, HTTP and HTTPS and deny everything else")
		return nil
	}

	port := c.sshPort()
	logx.Step("allowing SSH on port %s before enabling the firewall", port)
	if err := c.Runner.Run(c.Context, "ufw", "allow", port+"/tcp", "comment", "SSH"); err != nil {
		return fmt.Errorf("could not add the SSH rule; refusing to enable the firewall: %w", err)
	}
	for _, rule := range []string{"80/tcp", "443/tcp"} {
		c.Runner.TryRun(c.Context, "ufw", "allow", rule)
	}
	if c.Config.PhpMyAdmin.Enabled {
		for _, cidr := range c.Config.PhpMyAdmin.AllowList {
			c.Runner.TryRun(c.Context, "ufw", "allow", "from", cidr, "to", "any",
				"port", fmt.Sprint(c.Config.PhpMyAdmin.Port), "proto", "tcp")
		}
	}

	c.Runner.TryRun(c.Context, "ufw", "default", "deny", "incoming")
	c.Runner.TryRun(c.Context, "ufw", "default", "allow", "outgoing")
	// --force skips the "this may disrupt existing connections" prompt, which
	// nothing is present to answer.
	if err := c.Runner.Run(c.Context, "ufw", "--force", "enable"); err != nil {
		return err
	}
	logx.Change("firewall enabled: SSH on %s, HTTP, HTTPS; everything else denied", port)
	return nil
}

// sshPort reads the configured SSH port so the firewall rule matches reality
// rather than assuming 22.
func (c *Ctx) sshPort() string {
	out, err := c.Runner.Output(c.Context, "sshd", "-T")
	if err == nil {
		for _, line := range strings.Split(out, "\n") {
			if fields := strings.Fields(line); len(fields) == 2 && fields[0] == "port" {
				return fields[1]
			}
		}
	}
	return "22"
}

// applyUnattendedUpgrades turns on automatic security patching.
//
// Only the security pocket is enabled. Unattended upgrades from the general
// archive would move PHP or MariaDB versions underneath a running site.
func (c *Ctx) applyUnattendedUpgrades() {
	if !system.PackageInstalled(c.Context, c.Runner, "unattended-upgrades") {
		logx.Warn("unattended-upgrades is not installed; security patches will not be applied automatically")
		return
	}
	content := tmpl.ManagedHeader + `
//
// Automatic security updates only. Enabling the full archive here would let an
// unattended run change PHP or database versions under a live site.

APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Unattended-Upgrade "1";
APT::Periodic::AutocleanInterval "7";

Unattended-Upgrade::Automatic-Reboot "false";
Unattended-Upgrade::Remove-Unused-Kernel-Packages "true";
Unattended-Upgrade::Remove-Unused-Dependencies "true";
`
	if _, err := c.Writer.Write("/etc/apt/apt.conf.d/51ngxsetup-unattended", []byte(content), 0o644, false); err != nil {
		logx.Warn("could not configure unattended-upgrades: %v", err)
		return
	}
	logx.Change("automatic security updates enabled (reboots are left to you)")
}

// reportSSHConfiguration inspects SSH and reports weaknesses without changing
// anything.
//
// Rewriting sshd_config during an automated provision is how people lock
// themselves out of servers. These are findings for the operator to act on.
func (c *Ctx) reportSSHConfiguration() {
	out, err := c.Runner.Output(c.Context, "sshd", "-T")
	if err != nil {
		return
	}
	settings := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		if k, v, ok := strings.Cut(strings.TrimSpace(line), " "); ok {
			settings[k] = v
		}
	}
	if settings["permitrootlogin"] == "yes" {
		logx.Warn("SSH permits root login with a password. Consider `PermitRootLogin prohibit-password` once your key works.")
	}
	if settings["passwordauthentication"] == "yes" {
		logx.Warn("SSH accepts password authentication, which is what the fail2ban sshd jail spends its time blocking. Key-only authentication is stronger.")
	}
}

// phpMyAdmin paths.
const (
	pmaDir      = "/usr/share/phpmyadmin"
	pmaSlug     = "ngxsetup-tools"
	pmaHtpasswd = "/etc/nginx/ngxsetup-phpmyadmin.htpasswd"
	pmaVersion  = "5.2.2"
)

// ApplyPhpMyAdmin configures the optional database console, or removes it.
//
// When enabled it gets its own port, its own PHP pool running as its own user,
// an address allowlist and an HTTP credential in front of its login form. That
// is a deliberate contrast with the previous stack, which mounted it at /mysql
// on every site with no authentication of any kind.
func (c *Ctx) ApplyPhpMyAdmin() error {
	confPath := SitesAvailable + "/phpmyadmin.conf"
	linkPath := SitesEnabled + "/phpmyadmin.conf"

	if !c.Config.PhpMyAdmin.Enabled {
		if err := c.Writer.Remove(linkPath); err != nil {
			return err
		}
		if err := c.Writer.Remove(confPath); err != nil {
			return err
		}
		return c.Writer.Remove(fmt.Sprintf("/etc/php/%s/fpm/pool.d/%s.conf", c.PHPVersion, pmaSlug))
	}
	// Validate() already rejects an empty allow list, but this is the code
	// path that would expose a database console to the internet, so it is
	// checked again here.
	if len(c.Config.PhpMyAdmin.AllowList) == 0 {
		return fmt.Errorf("phpmyadmin requires an allow list")
	}
	if _, err := osStat(c.Path(pmaHtpasswd)); err != nil {
		return fmt.Errorf("no phpMyAdmin credential is set; run `ngxsetup secure phpmyadmin --user <name>` first")
	}

	if err := c.installPhpMyAdmin(); err != nil {
		return err
	}
	if err := c.writeToolsPool(); err != nil {
		return err
	}

	body, err := tmpl.Render("nginx/sites/phpmyadmin.conf.tmpl", tmpl.PhpMyAdmin{
		Port:         c.Config.PhpMyAdmin.Port,
		AllowList:    c.Config.PhpMyAdmin.AllowList,
		HtpasswdPath: pmaHtpasswd,
		SocketPath:   c.SocketPath(pmaSlug),
	})
	if err != nil {
		return err
	}
	if _, err := c.Writer.Write(confPath, body, 0o644, false); err != nil {
		return err
	}
	return c.Writer.Symlink(confPath, linkPath)
}

// SetPhpMyAdminCredential creates the HTTP credential guarding the console.
//
// The hash is produced by openssl in SHA-512 crypt form, which nginx accepts
// through crypt(3). The password is supplied on stdin so it never reaches a
// process listing.
func (c *Ctx) SetPhpMyAdminCredential(username, password string) error {
	if username == "" {
		return fmt.Errorf("a username is required")
	}
	if len(password) < 12 {
		return fmt.Errorf("the phpMyAdmin password must be at least 12 characters")
	}
	if !c.Runner.Look("openssl") {
		return fmt.Errorf("openssl is required to hash the password")
	}
	hash, err := c.Runner.RunStdin(c.Context, password+"\n", "openssl", "passwd", "-6", "-stdin")
	if err != nil {
		return err
	}
	line := fmt.Sprintf("%s:%s\n", username, strings.TrimSpace(hash))
	// Readable by nginx's master process only; it is a password database.
	if _, err := c.Writer.Write(pmaHtpasswd, []byte(line), 0o640, false); err != nil {
		return err
	}
	logx.Change("phpMyAdmin credential set for %s", username)
	return nil
}

func (c *Ctx) installPhpMyAdmin() error {
	if _, err := osStat(c.Path(pmaDir)); err == nil {
		logx.Skip("phpMyAdmin already present at %s", pmaDir)
		return nil
	}
	if c.Writer.DryRun {
		logx.Change("[dry-run] would install phpMyAdmin %s", pmaVersion)
		return nil
	}
	url := fmt.Sprintf("https://files.phpmyadmin.net/phpMyAdmin/%s/phpMyAdmin-%s-all-languages.tar.gz",
		pmaVersion, pmaVersion)
	tarball := "/tmp/phpmyadmin.tar.gz"
	defer osRemove(tarball)

	logx.Step("downloading phpMyAdmin %s", pmaVersion)
	if err := download(url, tarball, 0o600, 64<<20); err != nil {
		return err
	}
	if err := extractTarGz(tarball, c.Path(pmaDir), 1); err != nil {
		return err
	}
	logx.Change("installed phpMyAdmin to %s", pmaDir)
	return nil
}

// writeToolsPool gives phpMyAdmin its own PHP pool and user, so a flaw in it
// cannot read any site's files.
func (c *Ctx) writeToolsPool() error {
	user := "web-" + pmaSlug
	if err := system.EnsureSystemUser(c.Context, c.Runner, user, pmaDir); err != nil {
		return err
	}
	tmpDir := "/var/lib/ngxsetup/tools/tmp"
	sessDir := "/var/lib/ngxsetup/tools/sessions"
	for _, d := range []string{tmpDir, sessDir} {
		if err := c.Writer.EnsureDir(d, 0o700, user); err != nil {
			return err
		}
	}
	body, err := tmpl.Render("php/pool.conf.tmpl", tmpl.Pool{
		Plan: c.Plan, Slug: pmaSlug, Domain: "phpMyAdmin",
		User: user, Group: user,
		SocketPath: c.SocketPath(pmaSlug), Root: pmaDir,
		TmpDir: tmpDir, SessionDir: sessDir,
		TLS: false, StrictFunctions: true,
	})
	if err != nil {
		return err
	}
	_, err = c.Writer.Write(fmt.Sprintf("/etc/php/%s/fpm/pool.d/%s.conf", c.PHPVersion, pmaSlug), body, 0o644, false)
	return err
}

// Thin wrappers so the filesystem calls in this file read consistently with
// the rest of the package.
func osStat(p string) (os.FileInfo, error) { return os.Stat(p) }
func osRemove(p string) error              { return os.Remove(p) }
