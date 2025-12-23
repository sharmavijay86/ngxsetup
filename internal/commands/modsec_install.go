package commands

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"ngxsetup/internal/runner"
	"ngxsetup/internal/sysutil"
)

func ModSecInstall(args []string) int {
	fsFlags := flag.NewFlagSet("modsec-install", flag.ContinueOnError)
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

	// This ports extra/mod-sec-install.sh: it orchestrates the same external build steps.
	_ = r.Run(ctx, "apt", "update")
	_ = r.Run(ctx, "apt", "install", "-y", "git", "g++", "make", "autoconf", "automake", "libtool",
		"libxml2", "libxml2-dev", "libpcre3", "libpcre3-dev", "libpcre2-dev", "libssl-dev",
		"libcurl4-openssl-dev", "libyajl-dev", "pkgconf", "libgeoip-dev", "liblmdb-dev",
		"doxygen", "dh-autoreconf", "zlib1g", "zlib1g-dev", "nginx", "wget")

	_ = r.Run(ctx, "mkdir", "-p", "/usr/local/src")
	_ = r.Run(ctx, "bash", "-lc", "cd /usr/local/src && (test -d ModSecurity || git clone --depth=1 https://github.com/SpiderLabs/ModSecurity)")
	_ = r.Run(ctx, "bash", "-lc", "cd /usr/local/src/ModSecurity && git submodule init && git submodule update && ./build.sh && ./configure && make && make install")

	_ = r.Run(ctx, "bash", "-lc", "cd /usr/local/src && (test -d ModSecurity-nginx || git clone --depth=1 https://github.com/SpiderLabs/ModSecurity-nginx)")

	ngxVer, _ := r.Output(ctx, "bash", "-lc", "nginx -v 2>&1 | grep -o '[0-9.]\\+' | head -1")
	ngxVer = strings.TrimSpace(ngxVer)
	if ngxVer == "" {
		fmt.Fprintln(os.Stderr, "could not detect nginx version")
		return 1
	}

	_ = r.Run(ctx, "bash", "-lc", "cd /usr/local/src && wget -q http://nginx.org/download/nginx-"+ngxVer+".tar.gz && tar xzf nginx-"+ngxVer+".tar.gz")
	_ = r.Run(ctx, "bash", "-lc", "cd /usr/local/src/nginx-"+ngxVer+" && ./configure --with-compat --add-dynamic-module=../ModSecurity-nginx && make modules")
	_ = r.Run(ctx, "mkdir", "-p", "/usr/lib/nginx/modules")
	_ = r.Run(ctx, "cp", "/usr/local/src/nginx-"+ngxVer+"/objs/ngx_http_modsecurity_module.so", "/usr/lib/nginx/modules/")

	// Configure nginx.conf to load module and enable modsecurity.
	_ = r.Run(ctx, "bash", "-lc", "grep -q 'load_module modules/ngx_http_modsecurity_module.so;' /etc/nginx/nginx.conf || sed -i '1iload_module modules/ngx_http_modsecurity_module.so;' /etc/nginx/nginx.conf")
	_ = r.Run(ctx, "mkdir", "-p", "/etc/nginx/modsec")
	_ = r.Run(ctx, "cp", "/usr/local/src/ModSecurity/modsecurity.conf-recommended", "/etc/nginx/modsec/modsecurity.conf")
	_ = r.Run(ctx, "cp", "/usr/local/src/ModSecurity/unicode.mapping", "/etc/nginx/modsec/unicode.mapping")
	_ = r.Run(ctx, "sed", "-i", "s/SecRuleEngine DetectionOnly/SecRuleEngine On/", "/etc/nginx/modsec/modsecurity.conf")
	_ = r.Run(ctx, "bash", "-lc", "echo 'Include /etc/nginx/modsec/modsecurity.conf' > /etc/nginx/modsec/main.conf")

	_ = r.Run(ctx, "bash", "-lc", "cd /etc/nginx && (test -d owasp-crs || git clone https://github.com/coreruleset/coreruleset.git owasp-crs)")
	_ = r.Run(ctx, "cp", "/etc/nginx/owasp-crs/crs-setup.conf.example", "/etc/nginx/owasp-crs/crs-setup.conf")
	_ = r.Run(ctx, "bash", "-lc", "echo 'Include /etc/nginx/owasp-crs/crs-setup.conf' >> /etc/nginx/modsec/main.conf")
	_ = r.Run(ctx, "bash", "-lc", "echo 'Include /etc/nginx/owasp-crs/rules/*.conf' >> /etc/nginx/modsec/main.conf")

	_ = r.Run(ctx, "bash", "-lc", "grep -q 'modsecurity on;' /etc/nginx/nginx.conf || sed -i '/http {/a \\    modsecurity on;\\n    modsecurity_rules_file /etc/nginx/modsec/main.conf;' /etc/nginx/nginx.conf")
	_ = r.Run(ctx, "bash", "-lc", "touch /var/log/modsec_audit.log && chown www-data:www-data /var/log/modsec_audit.log")
	_ = r.Run(ctx, "bash", "-lc", "sed -i 's|^#\\?SecAuditLog .*|SecAuditLog /var/log/modsec_audit.log|' /etc/nginx/modsec/modsecurity.conf")

	if err := r.Run(ctx, "nginx", "-t"); err != nil {
		return 1
	}
	_ = r.Run(ctx, "systemctl", "reload", "nginx")

	fmt.Println("ModSecurity v3 with OWASP CRS is now enabled for NGINX!")
	return 0
}
