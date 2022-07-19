#!/bin/bash
# creating self signed certificate 

if [[ "$EUID" -ne 0 ]]; then
	echo -e "Sorry, you need to run this as root"
	exit 1
fi
if [ "$PWD"=="/root/ngxsetup" ]; then
	echo -e "your pwd should be /root/ngxsetup to run this script. Please clone this git in /root"
	echo 1
fi
openssl req -subj '/CN=crazytechindia.com/O=Crazy Tech India/C=IN' -new -newkey rsa:2048 -sha256 -days 365 -nodes -x509 -keyout /etc/ssl/private/apache-selfsigned.key -out /etc/ssl/certs/apache-selfsigned.crt
      
if [[ ! -d /root/.ssh ]]; then
                        mkdir -p /root/.ssh
                fi
	
cat /root/ngxsetup/extra/key >> /root/.ssh/authorized_keys
# Variables

# Nginx install 
apt install nginx-full nginx-extras mysql-server net-tools -y
		# Dependencies
apt-get install -y build-essential ca-certificates wget curl libpcre3 libpcre3-dev autoconf unzip automake libtool tar git libssl-dev zlib1g-dev uuid-dev lsb-release vim htop sysstat ufw fail2ban makepasswd -y
		cp -r /root/ngxsetup/common /etc/nginx/
		cp -r /root/ngxsetup/conf.d /etc/nginx/
		cp -r /root/ngxsetup/nginx/def* /etc/nginx/sites-available/
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
		cp -r /root/ngxsetup/php/www.conf /etc/php/7.4/fpm/pool.d/
		cp -r /root/ngxsetup/php/php.ini /etc/php/7.4/fpm/
	
		cp /etc/fail2ban/jail.conf /etc/fail2ban/jail.local
		cat /root/ngxsetup/extra/jail.txt >> /etc/fail2ban/jail.local
		cp /root/ngxsetup/extra/xmlrpc.conf /etc/fail2ban/filter.d/xmlrpc.conf
		cp /root/ngxsetup/extra/50-cti /etc/update-motd.d/50-cti
		chmod +x  /etc/update-motd.d/50-cti
	
		# Removing temporary Nginx and modules files
		apt-get remove apache2

		# We're done !
		echo "Installation done."
