#!/bin/bash
set -e

# 1. Update and install dependencies
sudo apt update
sudo apt install -y git g++ make autoconf automake libtool \
  libxml2 libxml2-dev libpcre3 libpcre3-dev libpcre2-dev libssl-dev \
  libcurl4-openssl-dev libyajl-dev pkgconf libgeoip-dev liblmdb-dev \
  doxygen dh-autoreconf zlib1g zlib1g-dev nginx wget

# 2. Download and build ModSecurity library
cd /usr/local/src || sudo mkdir -p /usr/local/src && cd /usr/local/src
sudo git clone --depth=1 https://github.com/SpiderLabs/ModSecurity
cd ModSecurity
sudo git submodule init
sudo git submodule update
sudo ./build.sh
sudo ./configure
sudo make
sudo make install

# 3. Download ModSecurity-nginx connector
cd /usr/local/src
sudo git clone --depth=1 https://github.com/SpiderLabs/ModSecurity-nginx

# 4. Download matching NGINX source (based on installed version)
NGX_VER=$(nginx -v 2>&1 | grep -o '[0-9.]\+' | head -1)
cd /usr/local/src
sudo wget -q "http://nginx.org/download/nginx-$NGX_VER.tar.gz"
sudo tar xzf "nginx-$NGX_VER.tar.gz"

# 5. Build the ModSecurity dynamic module
cd "nginx-$NGX_VER"
sudo ./configure --with-compat --add-dynamic-module=../ModSecurity-nginx
sudo make modules

# 6. Install the compiled module
sudo cp objs/ngx_http_modsecurity_module.so /usr/lib/nginx/modules/

# 7. Configure NGINX to load ModSecurity module
if ! grep -q 'load_module modules/ngx_http_modsecurity_module.so;' /etc/nginx/nginx.conf; then
  sudo sed -i '1iload_module modules/ngx_http_modsecurity_module.so;' /etc/nginx/nginx.conf
fi

# 8. Prepare ModSecurity config
sudo mkdir -p /etc/nginx/modsec
sudo cp /usr/local/src/ModSecurity/modsecurity.conf-recommended /etc/nginx/modsec/modsecurity.conf
sudo cp /usr/local/src/ModSecurity/unicode.mapping /etc/nginx/modsec/unicode.mapping
sudo sed -i 's/SecRuleEngine DetectionOnly/SecRuleEngine On/' /etc/nginx/modsec/modsecurity.conf
echo 'Include /etc/nginx/modsec/modsecurity.conf' | sudo tee /etc/nginx/modsec/main.conf

# 9. Download and set up the OWASP CRS (Core Rule Set)
cd /etc/nginx
sudo git clone https://github.com/coreruleset/coreruleset.git owasp-crs
sudo cp owasp-crs/crs-setup.conf.example owasp-crs/crs-setup.conf
echo 'Include /etc/nginx/owasp-crs/crs-setup.conf' | sudo tee -a /etc/nginx/modsec/main.conf
echo 'Include /etc/nginx/owasp-crs/rules/*.conf' | sudo tee -a /etc/nginx/modsec/main.conf

# 10. Enable ModSecurity in NGINX
if ! grep -q 'modsecurity on;' /etc/nginx/nginx.conf; then
  sudo sed -i '/http {/a \    modsecurity on;\n    modsecurity_rules_file /etc/nginx/modsec/main.conf;' /etc/nginx/nginx.conf
fi

# 11. Create and set permissions for audit log file (optional, for logging)
sudo touch /var/log/modsec_audit.log
sudo chown www-data:www-data /var/log/modsec_audit.log
sudo sed -i 's|^#\?SecAuditLog .*|SecAuditLog /var/log/modsec_audit.log|' /etc/nginx/modsec/modsecurity.conf

# 12. Test and reload NGINX
sudo nginx -t && sudo systemctl reload nginx

echo
echo "✅ ModSecurity v3 with OWASP CRS is now enabled for NGINX!"
