#!/bin/bash

# Check if running as root
if [ "$EUID" -ne 0 ]; then 
    echo "Please run as root"
    exit 1
fi

# Get system resources
TOTAL_MEM_GB=$(free -g | grep Mem | awk '{print $2}')
TOTAL_CPU=$(nproc)
DATE=$(date +%Y%m%d_%H%M%S)

# Calculate optimal values
BUFFER_POOL_SIZE=$(echo "$TOTAL_MEM_GB * 0.75" | bc | xargs printf "%.0f")
BUFFER_INSTANCES=$TOTAL_CPU
MAX_CONNECTIONS=$((TOTAL_MEM_GB * 100))
THREAD_CACHE=$((TOTAL_CPU * 12))

# Function to get value from existing mysqld.cnf
get_mysql_value() {
    local param=$1
    local default=$2
    local value=$(grep "^$param" /etc/mysql/mysql.conf.d/mysqld.cnf 2>/dev/null | awk '{print $NF}')
    echo ${value:-$default}
}

# Read existing basic settings
MYSQL_USER=$(get_mysql_value "user" "mysql")
MYSQL_PID_FILE=$(get_mysql_value "pid-file" "/var/run/mysqld/mysqld.pid")
MYSQL_SOCKET=$(get_mysql_value "socket" "/var/run/mysqld/mysqld.sock")
MYSQL_PORT=$(get_mysql_value "port" "3306")
MYSQL_DATADIR=$(get_mysql_value "datadir" "/var/lib/mysql")
MYSQL_BIND_ADDR=$(get_mysql_value "bind-address" "0.0.0.0")

# Backup existing configuration
echo "Backing up current MySQL configuration..."
cp /etc/mysql/mysql.conf.d/mysqld.cnf "/etc/mysql/mysql.conf.d/mysqld.cnf.backup.$DATE"

# Create new configuration
cat > /etc/mysql/mysql.conf.d/mysqld.cnf << EOF
[mysqld]
# Basic Settings
user                    = ${MYSQL_USER}
pid-file                = ${MYSQL_PID_FILE}
socket                  = ${MYSQL_SOCKET}
port                    = ${MYSQL_PORT}
datadir                 = ${MYSQL_DATADIR}
bind-address            = ${MYSQL_BIND_ADDR}

# Buffer Pool Settings
innodb_buffer_pool_size = ${BUFFER_POOL_SIZE}G
innodb_buffer_pool_instances = $BUFFER_INSTANCES

# InnoDB Settings
innodb_file_per_table  = 1
innodb_flush_method    = O_DIRECT
innodb_log_file_size   = 2G
innodb_log_buffer_size = 128M
innodb_write_io_threads = $TOTAL_CPU
innodb_read_io_threads  = $TOTAL_CPU
innodb_io_capacity     = 2000
innodb_io_capacity_max = 4000
innodb_flush_log_at_trx_commit = 2
innodb_lock_wait_timeout = 120
innodb_ft_min_token_size = 2
innodb_ft_enable_stopword = 0

# Connection Settings
max_connections        = $MAX_CONNECTIONS
thread_cache_size     = $THREAD_CACHE
table_open_cache      = 8000
table_open_cache_instances = $TOTAL_CPU
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
EOF

# Configure system settings
cat > /etc/sysctl.d/99-mysql.conf << EOF
vm.swappiness = 1
vm.dirty_background_ratio = 2
vm.dirty_ratio = 40
net.core.somaxconn = 65535
net.ipv4.tcp_max_syn_backlog = 65535
fs.aio-max-nr = 1048576
fs.file-max = 6815744
EOF

# Apply system settings
sysctl -p /etc/sysctl.d/99-mysql.conf

# Install monitoring tools
if [[ "$OSTYPE" == "darwin"* ]]; then
    brew install percona-toolkit mytop
else
    apt-get update
    apt-get install -y percona-toolkit mytop
fi

# Download MySQLTuner
curl -L https://raw.githubusercontent.com/major/MySQLTuner-perl/master/mysqltuner.pl -o /usr/local/bin/mysqltuner.pl
chmod +x /usr/local/bin/mysqltuner.pl

# Restart MySQL
echo "Restarting MySQL..."
systemctl restart mysql

# Verify configuration
echo "Waiting for MySQL to start..."
sleep 10
if systemctl is-active --quiet mysql; then
    echo "MySQL successfully restarted"
    echo "Configuration complete! New config saved at /etc/mysql/mysql.conf.d/mysqld.cnf"
    echo "Backup saved at /etc/mysql/mysql.conf.d/mysqld.cnf.backup.$DATE"
    echo "Run 'mysqltuner.pl' after 48 hours for performance analysis"
else
    echo "Error: MySQL failed to start. Rolling back changes..."
    cp "/etc/mysql/mysql.conf.d/mysqld.cnf.backup.$DATE" /etc/mysql/mysql.conf.d/mysqld.cnf
    systemctl restart mysql
fi
