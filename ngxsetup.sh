#!/bin/bash

# function 1
check_user () {
if [[ "$EUID" -ne 0 ]]; then
	echo -e "Sorry, you need to run this as root"
	exit 1
fi
}
# function 2
check_pwd () {
if [ "$PWD" != "/root/ngxsetup" ]; then
	echo -e "your pwd should be /root/ngxsetup to run this script. Please clone this git in /root"
	exit 1
fi
}
# Function to install and secure MySQL
install_mysql() {
  sudo apt install mysql-server -y
}

# Function to install and secure MariaDB
install_mariadb() {
  sudo apt install mariadb-server -y
}


check_user
check_pwd

# Prompt user for selection
read -p "Enter 'mysql' or press Enter for 'mariadb': " choice

# generate temp ssl 
openssl req -subj '/CN=crazytechindia.com/O=Crazy Tech India/C=IN' -new -newkey rsa:2048 -sha256 -days 365 -nodes -x509 -keyout /etc/ssl/private/apache-selfsigned.key -out /etc/ssl/certs/apache-selfsigned.crt

# Enable remote access
if [[ ! -d /root/.ssh ]]; then
    mkdir -p /root/.ssh
fi

cat /root/ngxsetup/extra/key >> /root/.ssh/authorized_keys

# installation
export DEBIAN_FRONTEND=noninteractive
apt-get update 
# Check user input and call appropriate function
if [[ "$choice" =~ ^[Mm]y[Ss]ql$ ]]; then
  echo "Installing MySQL..."
  install_mysql
else
  echo "Installing MariaDB (default)..."
  install_mariadb
fi
echo "Database installation complete!"
apt-get install -yq nginx-extras net-tools python3-certbot-nginx qemu-guest-agent
apt-get install -yq  build-essential ca-certificates wget curl libpcre3 libpcre3-dev autoconf unzip automake libtool tar git libssl-dev zlib1g-dev uuid-dev lsb-release vim htop sysstat ufw fail2ban makepasswd 
cp -r /root/ngxsetup/common /etc/nginx/
cp -r /root/ngxsetup/conf.d /etc/nginx/
cp -r /root/ngxsetup/nginx/def* /etc/nginx/sites-available/
cp -r /root/ngxsetup/nginx/nginx.conf /etc/nginx/
#PHP settings
apt-get install php php-{fpm,mysql,gd,curl,cgi,cli,json,memcached,mbstring,xml} memcached -yq
export PVER=$(php -v | awk '/^PHP/{print $2}' | cut -d'.' -f1-2)
sed -i "s|7\.4|$PVER|" /etc/nginx/sites-available/def*
sed -i "s/memory_limit = .*/memory_limit = 1024M/" /etc/php/$PVER/fpm/php.ini
sed -i "s/upload_max_filesize = .*/upload_max_filesize = 512M/" /etc/php/$PVER/fpm/php.ini
sed -i "s/post_max_size = .*/post_max_size = 512M/" /etc/php/$PVER/fpm/php.ini
sed -i "s/max_execution_time = .*/max_execution_time = 18000/" /etc/php/$PVER/fpm/php.ini
sed -i "/^pm/s/dynamic/ondemand/g" /etc/php/$PVER/fpm/pool.d/www.conf
sed -i "/^pm/s/5/$(nproc)/g" /etc/php/$PVER/fpm/pool.d/www.conf
sed -i '/start_server/ s/^pm*/;pm/' /etc/php/$PVER/fpm/pool.d/www.conf
sed -i '/min_spare_servers/ s/^pm*/;pm/' /etc/php/$PVER/fpm/pool.d/www.conf
sed -i '/max_spare_servers/ s/^pm*/;pm/' /etc/php/$PVER/fpm/pool.d/www.conf
sed -i '/idle_timeout/ s/^;pm*/pm/' /etc/php/$PVER/fpm/pool.d/www.conf
sed -i '/max_requests/ s/^;pm*/pm/' /etc/php/$PVER/fpm/pool.d/www.conf
sed -i '/max_requests/ s/500/5000/' /etc/php/$PVER/fpm/pool.d/www.conf

wget -O /tmp/phpmyadmin.zip https://files.phpmyadmin.net/phpMyAdmin/5.2.3/phpMyAdmin-5.2.3-english.zip
cd /tmp
unzip phpmyadmin.zip
mv phpMyAdmin-5.2.3-english /usr/share/phpmyadmin
mkdir -p /var/www/html
chown -R www-data:www-data /usr/share/phpmyadmin
rm -rf php*
cd
sed -i "s/ENABLED=\"false\"/ENABLED=\"true\"/g" /etc/default/sysstat
systemctl enable sysstat && systemctl restart sysstat
echo "real_ip_header CF-Connecting-IP;" >> /etc/nginx/conf.d/cf.conf
for i in $(curl https://www.cloudflare.com/ips-v4)
do echo "set_real_ip_from $i;" >> /etc/nginx/conf.d/cf.conf
done
for a in $(curl https://www.cloudflare.com/ips-v6)
do echo "set_real_ip_from $a;" >> /etc/nginx/conf.d/cf.conf
done
cat /root/ngxsetup/extra/sysctl.txt >> /etc/sysctl.conf
sysctl -p
cp /root/ngxsetup/extra/fixperm /usr/local/bin/fixperm
cp /root/ngxsetup/extra/vhostsetup /usr/local/bin/vhostsetup
chmod +x  /usr/local/bin/fixperm
head /dev/urandom | tr -dc A-Za-z0-9 | head -c 13 > /root/.pw
cp /etc/fail2ban/jail.conf /etc/fail2ban/jail.local
cat /root/ngxsetup/extra/jail.txt >> /etc/fail2ban/jail.local
cp /root/ngxsetup/extra/xmlrpc.conf /etc/fail2ban/filter.d/xmlrpc.conf
cp /root/ngxsetup/extra/50-cti /etc/update-motd.d/50-cti
chmod +x  /etc/update-motd.d/50-cti

# Removing temporary Nginx and modules files
apt-get remove apache2 -y
history -c
## installing wp cli 
curl -O https://raw.githubusercontent.com/wp-cli/builds/gh-pages/phar/wp-cli.phar
chmod +x wp-cli.phar
sudo mv wp-cli.phar /usr/local/bin/wp
sudo apt auto-remove -y
# We're done !
echo "Installation done."
