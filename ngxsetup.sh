#!/bin/bash
# Variables here
VER=$(lsb_release  -r | awk '{print $2}' | cut -d . -f1)

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

check_user
check_pwd

# generate temp ssl 
openssl req -subj '/CN=crazytechindia.com/O=Crazy Tech India/C=IN' -new -newkey rsa:2048 -sha256 -days 365 -nodes -x509 -keyout /etc/ssl/private/apache-selfsigned.key -out /etc/ssl/certs/apache-selfsigned.crt

# Enable remote access
if [[ ! -d /root/.ssh ]]; then
    mkdir -p /root/.ssh
fi

cat /root/ngxsetup/extra/key >> /root/.ssh/authorized_keys

# installation
apt-get update 
apt-get install -y nginx-extras mysql-server net-tools 
apt-get install -y  build-essential ca-certificates wget curl libpcre3 libpcre3-dev autoconf unzip automake libtool tar git libssl-dev zlib1g-dev uuid-dev lsb-release vim htop sysstat ufw fail2ban makepasswd 
cp -r /root/ngxsetup/common /etc/nginx/
cp -r /root/ngxsetup/conf.d /etc/nginx/
cp -r /root/ngxsetup/nginx/def* /etc/nginx/sites-available/

#for ubuntu 22.04 with php8.1 version
if [ $VER -eq 22 ]
then 
sed -i "s|7\.4|8\.1|" /etc/nginx/sites-available/def*
fi

apt-get install php php-{fpm,mysql,gd,curl,cgi,cli,json,memcached,mbstring,xml} memcached -y
wget -O /tmp/phpmyadmin.zip https://files.phpmyadmin.net/phpMyAdmin/5.2.0/phpMyAdmin-5.2.0-english.zip
cd /tmp
unzip phpmyadmin.zip
mv phpMyAdmin-5.2.0-english /usr/share/phpmyadmin
mkdir -p /var/www/html
ln -s /usr/share/phpmyadmin /var/www/html/phpmyadmin
rm -rf php*
cd
sed -i "s/ENABLED=\"false\"/ENABLED=\"true\"/g" /etc/default/sysstat
systemctl restart sysstat
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

# We're done !
echo "Installation done."
