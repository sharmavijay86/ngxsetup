#!/bin/bash

# Global variables
LOGFILE="/var/log/ngxsetup.log"
PHP_VERSION=$(php -v | awk '/^PHP/{print $2}' | cut -d'.' -f1-2)
MYSQL_ROOT_PASS=$(openssl rand -base64 24)
PHPMYADMIN_VERSION="5.2.0"

# Enhanced logging function
log() {
    local level=$1
    shift
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] [$level] $*" | tee -a "$LOGFILE"
}

# Error handling
set -eE
trap 'log ERROR "Script failed on line $LINENO"' ERR

# Function to check requirements
check_requirements() {
    log INFO "Checking system requirements..."
    if [[ "$EUID" -ne 0 ]]; then
        log ERROR "Must run as root"
        exit 1
    }

    # Check minimum system requirements
    local min_ram=1024  # 1GB
    local ram_mb=$(free -m | awk '/^Mem:/{print $2}')
    if (( ram_mb < min_ram )); then
        log ERROR "Insufficient RAM. Minimum ${min_ram}MB required"
        exit 1
    fi
}

# Function to install and secure MySQL/MariaDB
install_database() {
    local db_type=$1
    log INFO "Installing $db_type..."
    
    export DEBIAN_FRONTEND=noninteractive
    
    if [[ "$db_type" == "mysql" ]]; then
        apt-get install -y mysql-server || {
            log ERROR "MySQL installation failed"
            exit 1
        }
        # Secure MySQL installation
        mysql -e "ALTER USER 'root'@'localhost' IDENTIFIED WITH mysql_native_password BY '${MYSQL_ROOT_PASS}';"
    else
        apt-get install -y mariadb-server || {
            log ERROR "MariaDB installation failed"
            exit 1
        }
        # Secure MariaDB installation
        mysql -e "UPDATE mysql.user SET Password=PASSWORD('${MYSQL_ROOT_PASS}') WHERE User='root';"
    fi

    mysql -e "DELETE FROM mysql.user WHERE User='';"
    mysql -e "DELETE FROM mysql.user WHERE User='root' AND Host NOT IN ('localhost', '127.0.0.1', '::1');"
    mysql -e "DROP DATABASE IF EXISTS test;"
    mysql -e "DELETE FROM mysql.db WHERE Db='test' OR Db='test\\_%';"
    mysql -e "FLUSH PRIVILEGES;"
    
    log INFO "Database secured successfully"
}

# Function to install and configure PHP
install_php() {
    log INFO "Installing PHP and extensions..."
    
    apt-get install -y \
        php php-fpm php-mysql php-gd php-curl php-cgi \
        php-cli php-json php-memcached php-mbstring php-xml \
        memcached || {
        log ERROR "PHP installation failed"
        exit 1
    }

    # Configure PHP
    local php_ini="/etc/php/${PHP_VERSION}/fpm/php.ini"
    local php_fpm_conf="/etc/php/${PHP_VERSION}/fpm/pool.d/www.conf"

    sed -i 's/^memory_limit.*/memory_limit = 1024M/' "$php_ini"
    sed -i 's/^upload_max_filesize.*/upload_max_filesize = 512M/' "$php_ini"
    sed -i 's/^post_max_size.*/post_max_size = 512M/' "$php_ini"
    sed -i 's/^max_execution_time.*/max_execution_time = 18000/' "$php_ini"

    # Optimize PHP-FPM
    local cpu_cores=$(nproc)
    sed -i "s/^pm = .*/pm = ondemand/" "$php_fpm_conf"
    sed -i "s/^pm.max_children = .*/pm.max_children = $((cpu_cores * 50))/" "$php_fpm_conf"
    sed -i "s/^pm.max_requests = .*/pm.max_requests = 5000/" "$php_fpm_conf"
    sed -i "s/^;request_terminate_timeout = .*/request_terminate_timeout = 300s/" "$php_fpm_conf"
    sed -i "s/^;rlimit_files = .*/rlimit_files = 131072/" "$php_fpm_conf"
    
    # If the directives don't exist or are commented differently, append them
    if ! grep -q "request_terminate_timeout = " "$php_fpm_conf"; then
      echo "request_terminate_timeout = 300s" >> "$php_fpm_conf"
    fi
    if ! grep -q "rlimit_files = " "$php_fpm_conf"; then
      echo "rlimit_files = 131072" >> "$php_fpm_conf"
    fi
}

# Function to install and configure Nginx
install_nginx() {
    log INFO "Installing Nginx..."
    
    apt-get install -y nginx-extras || {
        log ERROR "Nginx installation failed"
        exit 1
    }

    # Copy configuration files
    cp -r /root/ngxsetup/common /etc/nginx/
    cp -r /root/ngxsetup/conf.d /etc/nginx/
    cp -r /root/ngxsetup/nginx/def* /etc/nginx/sites-available/
    cp -r /root/ngxsetup/nginx/nginx.conf /etc/nginx/

    # Configure CloudFlare IPs
    configure_cloudflare
}

# Function to configure CloudFlare IPs
configure_cloudflare() {
    log INFO "Configuring CloudFlare IPs..."
    echo "real_ip_header CF-Connecting-IP;" >> /etc/nginx/conf.d/cf.conf
    for i in $(curl https://www.cloudflare.com/ips-v4)
    do echo "set_real_ip_from $i;" >> /etc/nginx/conf.d/cf.conf
    done
    for a in $(curl https://www.cloudflare.com/ips-v6)
    do echo "set_real_ip_from $a;" >> /etc/nginx/conf.d/cf.conf
    done
}

# Function to install WP CLI
install_wp_cli() {
    log INFO "Installing WP CLI..."
    curl -O https://raw.githubusercontent.com/wp-cli/builds/gh-pages/phar/wp-cli.phar
    chmod +x wp-cli.phar
    mv wp-cli.phar /usr/local/bin/wp
}

# Function to configure security settings
configure_security() {
    log INFO "Configuring security settings..."
    cp /etc/fail2ban/jail.conf /etc/fail2ban/jail.local
    cat /root/ngxsetup/extra/jail.txt >> /etc/fail2ban/jail.local
    cp /root/ngxsetup/extra/xmlrpc.conf /etc/fail2ban/filter.d/xmlrpc.conf
}

# Function to configure system settings
configure_system() {
    log INFO "Configuring system settings..."
    
    # Create sysctl configuration
    cat > /etc/sysctl.d/99-wordpress-performance.conf << 'EOF'
# Network Settings
net.core.somaxconn = 65536
net.core.netdev_max_backlog = 65536
net.ipv4.tcp_max_syn_backlog = 65536
net.ipv4.tcp_fin_timeout = 10
net.ipv4.tcp_keepalive_time = 1200
net.ipv4.tcp_max_tw_buckets = 1440000
net.ipv4.ip_local_port_range = 1024 65535
net.ipv4.tcp_slow_start_after_idle = 0
net.ipv4.tcp_window_scaling = 1
net.ipv4.tcp_timestamps = 1

# File System
fs.file-max = 2097152
fs.inotify.max_user_watches = 524288
fs.aio-max-nr = 1048576

# VM Settings
vm.swappiness = 10
vm.dirty_ratio = 60
vm.dirty_background_ratio = 2
vm.vfs_cache_pressure = 50
vm.zone_reclaim_mode = 0
vm.max_map_count = 262144
EOF

    # Apply sysctl settings
    sysctl -p /etc/sysctl.d/99-wordpress-performance.conf || {
        log ERROR "Failed to apply sysctl settings"
        exit 1
    }

    # Configure system limits
    cat > /etc/security/limits.d/wordpress.conf << 'EOF'
# Increase limits for web server user
www-data soft nofile 100000
www-data hard nofile 100000
www-data soft nproc 65535
www-data hard nproc 65535

# Increase limits for MySQL user
mysql soft nofile 100000
mysql hard nofile 100000
mysql soft nproc 65535
mysql hard nproc 65535
EOF

    # Enable limits
    sed -i 's/^# *session *required *pam_limits.so/session required pam_limits.so/' /etc/pam.d/common-session

    # Configure I/O scheduler for SSDs
    for disk in $(lsblk -d -o name | grep -v loop | grep -v name); do
        if [ -f "/sys/block/$disk/queue/rotational" ] && [ "$(cat /sys/block/$disk/queue/rotational)" -eq 0 ]; then
            echo "none" > "/sys/block/$disk/queue/scheduler"
            log INFO "Set I/O scheduler to none for SSD device $disk"
        fi
    done

    # Configure transparent hugepages
    if [ -f "/sys/kernel/mm/transparent_hugepage/enabled" ]; then
        echo never > /sys/kernel/mm/transparent_hugepage/enabled
        echo "transparent_hugepages=never" >> /etc/default/grub
        update-grub
    fi

    # Configure systemd service limits
    mkdir -p /etc/systemd/system.conf.d/
    cat > /etc/systemd/system.conf.d/limits.conf << 'EOF'
[Manager]
DefaultLimitNOFILE=100000
DefaultLimitNPROC=65535
EOF

    # Reload systemd
    systemctl daemon-reload

    # Set permissions for web directories
    cp /root/ngxsetup/extra/fixperm /usr/local/bin/fixperm
    chmod +x /usr/local/bin/fixperm

    # Configure virtual host setup script
    cp /root/ngxsetup/extra/vhostsetup /usr/local/bin/vhostsetup
    chmod +x /usr/local/bin/vhostsetup

    log INFO "System configuration completed successfully"
}

# Main execution
main() {
    check_requirements
    
    log INFO "Starting installation..."
    
    # Update system
    apt-get update && apt-get upgrade -y

    # Install database
    read -p "Enter 'mysql' or press Enter for 'mariadb': " db_choice
    install_database "${db_choice:-mariadb}"
    
    # Install core components
    install_php
    install_nginx
    install_wp_cli
    
    # Final configurations
    configure_security
    configure_system
    
    log INFO "Installation completed successfully"
    echo "MySQL root password: $MYSQL_ROOT_PASS"
    echo "Installation log available at: $LOGFILE"
}

# Run main function
main "$@"
