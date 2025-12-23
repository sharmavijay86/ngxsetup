# INSTALL / Usage (single-binary)

This project ships as a **single static Linux binary** that embeds the required repo assets (`common/`, `conf.d/`, `nginx/`, `extra/`, `docs/`).

The binary provides a Go CLI for the full setup workflow:
- `setup`
- `vhostsetup`
- `fixperm`
- `loadcheck`
- `mysqltune`
- `modsec-install`

## Safety / what this installer changes

Running `setup` is **not** a “dry config generator” — it applies changes to the machine.

It will (high-level):
- Install packages via `apt-get` (nginx, php-fpm, db server, fail2ban, sysstat, certbot, etc.)
- Write and overwrite Nginx config files under `/etc/nginx` (including `nginx.conf`, `sites-available/def*`, `common/`, `conf.d/`)
- Tune PHP-FPM settings under `/etc/php/<version>/fpm/`
- Download and install phpMyAdmin to `/usr/share/phpmyadmin`
- Create a self-signed TLS cert at:
  - `/etc/ssl/certs/apache-selfsigned.crt`
  - `/etc/ssl/private/apache-selfsigned.key`
- Append an embedded SSH public key to `/root/.ssh/authorized_keys`
- Write Cloudflare real-IP config to `/etc/nginx/conf.d/cf.conf`
- Append sysctl settings from the embedded `extra/sysctl.txt` into `/etc/sysctl.conf` and apply them
- Configure fail2ban config additions (jail.local + xmlrpc filter)
- Remove `apache2`
- Install WP-CLI to `/usr/local/bin/wp`

If you’re testing, prefer running on a fresh Ubuntu VM.

## Requirements (target machine)

- Ubuntu / Debian-family system with `apt-get`
- Run as `root` (or `sudo -i`)
- Network access (downloads phpMyAdmin, WordPress, Cloudflare IP lists, WP-CLI phar)

> Note: `vhostsetup` can run `certbot` for Let’s Encrypt when you choose the SSL + WordPress option.

## Install the binary (recommended)

1) Copy the Linux binary onto the server (example):

```bash
scp ./dist/ngxsetup-linux-amd64 root@YOUR_SERVER:/root/
```

2) SSH in:

```bash
ssh root@YOUR_SERVER
```

3) Make executable:

```bash
chmod +x /root/ngxsetup-linux-amd64
```

4) (Optional but recommended) do a dry run first:

```bash
/root/ngxsetup-linux-amd64 setup --dry-run
```

5) Run setup (interactive DB prompt):

```bash
/root/ngxsetup-linux-amd64 setup
```

Or choose DB explicitly:

```bash
/root/ngxsetup-linux-amd64 setup --db=mariadb
# or
/root/ngxsetup-linux-amd64 setup --db=mysql
```

### What “installation” means

During `setup`, the binary will install itself to:

- `/usr/local/bin/ngxsetup`

…and create symlinks for compatibility:

- `/usr/local/bin/vhostsetup`
- `/usr/local/bin/fixperm`
- `/usr/local/bin/loadcheck`
- `/usr/local/bin/mysqltune`
- `/usr/local/bin/modsec-install`

After that you can run `vhostsetup` directly like the original system.

## CLI usage

### Global help

```bash
ngxsetup --help
ngxsetup help
```

### Setup

```bash
ngxsetup setup [--db=mariadb|mysql] [--dry-run]
```

Flags:
- `--db=`: `mariadb` (default) or `mysql`
- `--dry-run`: prints the commands it would run

### vhostsetup

Interactive:

```bash
vhostsetup
# or
ngxsetup vhostsetup
```

Menu options:
- `v`  : create non-SSL vhost (no WordPress)
- `vs` : create SSL vhost (no WordPress)
- `w`  : create non-SSL vhost + download WordPress + create DB/user
- `ws` : create SSL vhost + download WordPress + create DB/user + run WP-CLI + certbot

Outputs:
- A per-site info file is written under `/root/<domainWithoutDots>` with document root and DB credentials.
- Document root is `/var/www/<domainWithoutDots>`.
- Nginx site is created in `/etc/nginx/sites-enabled/<domainWithoutDots>` from either:
  - `/etc/nginx/sites-available/default`
  - `/etc/nginx/sites-available/defaultssl`

### fixperm

```bash
fixperm
# or
ngxsetup fixperm
```

Sets ownership of `/var/www/*/wp-content` to `www-data:www-data`.

### loadcheck

```bash
loadcheck
# or
ngxsetup loadcheck
```

Stops nginx if load average is greater than CPU cores; starts nginx if it’s down and load is OK.

### mysqltune

```bash
mysqltune
# or
ngxsetup mysqltune
```

Writes a tuned `/etc/mysql/mysql.conf.d/mysqld.cnf` based on system resources, applies sysctl, installs a couple of monitoring tools, and restarts MySQL.

### modsec-install

```bash
modsec-install
# or
ngxsetup modsec-install
```

Builds and enables ModSecurity v3 + OWASP CRS for the installed Nginx version (compiles a dynamic module).

## Build the Linux binary yourself (from this repo)

From a development machine with Go installed:

```bash
cd ngxsetup
mkdir -p dist
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o dist/ngxsetup-linux-amd64 ./cmd/ngxsetup
```

For ARM64:

```bash
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o dist/ngxsetup-linux-arm64 ./cmd/ngxsetup
```

## Troubleshooting

- If `vhostsetup` says default templates are missing, confirm these exist:
  - `/etc/nginx/sites-available/default`
  - `/etc/nginx/sites-available/defaultssl`

- If `ws` option fails during cert issuance, verify:
  - DNS for the domain points to this server
  - ports 80 and 443 are reachable
  - `certbot` is installed (`python3-certbot-nginx`)

- If MySQL commands fail during WordPress setup, the tool tries without a root password first; if that fails, it falls back to reading `/root/.pw`.
