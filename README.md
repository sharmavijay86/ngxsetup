# ngxsetup (Go-only)

Single-binary Ubuntu Nginx + MySQL/MariaDB + PHP setup.

This repo is **Go-only**: legacy bash scripts have been removed. The Go binary embeds the required Nginx/config assets from this repo and applies the same setup logic via a CLI.

## Install / usage

See [INSTALL.md](INSTALL.md).

Quick start on the server:

```bash
sudo ./ngxsetup-linux-amd64 setup
vhostsetup
```
