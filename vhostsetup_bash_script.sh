#!/bin/bash
clear
echo "############  Script to setup nginx vhost #############"
if [ ! -f /etc/nginx/sites-available/default ] || [ ! -f /etc/nginx/sites-available/defaultssl ]; then
    echo "config file does not exist contact helpdesk!"
    exit 0
fi
while true 
do
printf " Press v if you want to setup a new domain only \n"
printf " Press vs if you want to setup a new SSL domain only \n"
printf " Press w if you want to setup a new domain with wordpress \n"
printf " Press ws if you want to setup a new SSL domain with wordpress \n"
printf " Press x to exit \n"
read -p "Type your choice:" option
case $option in 
v|V)
 read -p "Give your domain name:" dn
 fname=`echo $dn | tr -d '.'`
 echo "your domain name is: $dn" >> /root/$fname
 echo "your document root path is: /var/www/$fname" >> /root/$fname
 sed  "s/localhost/$dn www.$dn/g"  /etc/nginx/sites-available/default > /etc/nginx/sites-enabled/$fname
 sed -i  "s#/www/html#/www/$fname#g" /etc/nginx/sites-enabled/$fname
 mkdir -p /var/www/$fname
 chown -R www-data:www-data /var/www/$fname
  clear
 echo "######################################################"
 printf "Domain has been setup put your files here at /var/www/$fname.\n"
 printf "Dont forget to run fixperm command to fix owenership.\n"
 printf " Need to setup another domain? please run vhostsetup command again! \n"
 echo "######################################################"

 exit 0
;;
w|W)
 read -p "Give your domain name:" dn
 fname=`echo $dn | tr -d '.'`
 echo "your domain name is: $dn" >> /root/$fname
 echo "your document root path is: /var/www/$fname" >> /root/$fname
 dbr=`echo $dn | tr -d '.' | cut -b 1-6`
 dbn=`echo $dbr$(cat /dev/urandom | tr -cd 'a-f0-9' | head -c 3)`
 head /dev/urandom | tr -dc A-Za-z0-9 | head -c 13  > /tmp/mdbpassw
 dpass=$(cat /tmp/mdbpassw)
 echo "Your database name is: $dbn" >> /root/$fname
 echo "Your database user is: $dbn" >> /root/$fname
 echo "Your database user password is: $dpass" >> /root/$fname
 sed  "s/localhost/$dn www.$dn/g"  /etc/nginx/sites-available/default > /etc/nginx/sites-enabled/$fname
 sed -i  "s#/www/html#/www/$fname#g" /etc/nginx/sites-enabled/$fname
 mysql -u root -p`cat /root/.pw` -e "CREATE DATABASE ${dbn}" 
 mysql -u root -p`cat /root/.pw` -e "GRANT ALL PRIVILEGES ON ${dbn}.* to '${dbn}'@'localhost' IDENTIFIED BY'${dpass}';"
 mysql -u root -p`cat /root/.pw` -e "FLUSH PRIVILEGES;"
 wget -O /tmp/wordpress.zip  https://wordpress.org/latest.zip 
 cd /tmp
 unzip -o /tmp/wordpress.zip 
 mv /tmp/wordpress /var/www/$fname
 clear
 echo "######################################################"
 printf "Open url $dn in your browser and install wordpress. \n read file /root/$fname for database information to be use during installation\n"
 printf " Need to setup another domain? please run vhostsetup command again! \n"
 echo "######################################################"
 cd
 rm -rf /tmp/mdbpassw /tmp/wordpress/ /tmp/latest.zip > /dev/null 2>&1
 chown -R www-data:www-data /var/www/$fname
 exit 0
;;
vs|VS)
 read -p "Give your domain name:" dn
 fname=`echo $dn | tr -d '.'`
 echo "your domain name is: $dn" >> /root/$fname
 echo "your document root path is: /var/www/$fname" >> /root/$fname
 sed  "s/localhost/$dn www.$dn/g"  /etc/nginx/sites-available/defaultssl > /etc/nginx/sites-enabled/$fname
 sed -i  "s#/www/html#/www/$fname#g" /etc/nginx/sites-enabled/$fname
 mkdir -p /var/www/$fname
 chown -R www-data:www-data /var/www/$fname
  clear
 echo "######################################################"
 printf "Domain has been setup put your files here at /var/www/$fname.\n"
 printf "Dont forget to run fixperm command to fix owenership.\n"
 printf " Need to setup another domain? please run vhostsetup command again! \n"
 echo "######################################################"

 exit 0
;;
 ws|WS)
 read -p "Give your domain name:" dn
 fname=`echo $dn | tr -d '.'`
 echo "your domain name is: $dn" >> /root/$fname
 echo "your document root path is: /var/www/$fname" >> /root/$fname
 dbr=`echo $dn | tr -d '.' | cut -b 1-6`
 dbn=`echo $dbr$(cat /dev/urandom | tr -cd 'a-f0-9' | head -c 3)`
 head /dev/urandom | tr -dc A-Za-z0-9 | head -c 13  > /tmp/mdbpassw
 dpass=$(cat /tmp/mdbpassw)
 echo "Your database name is: $dbn" >> /root/$fname
 echo "Your database user is: $dbn" >> /root/$fname
 echo "Your database user password is: $dpass" >> /root/$fname
 sed  "s/localhost/$dn www.$dn/g"  /etc/nginx/sites-available/defaultssl > /etc/nginx/sites-enabled/$fname
 sed -i  "s#/www/html#/www/$fname#g" /etc/nginx/sites-enabled/$fname
 mysql -u root -p`cat /root/.pw` -e "CREATE DATABASE ${dbn}"
 mysql -u root -p`cat /root/.pw` -e "GRANT ALL PRIVILEGES ON ${dbn}.* to '${dbn}'@'localhost' IDENTIFIED BY'${dpass}';"
 mysql -u root -p`cat /root/.pw` -e "FLUSH PRIVILEGES;"
 wget -O /tmp/wordpress.zip  https://wordpress.org/latest.zip
 cd /tmp
 unzip -o /tmp/wordpress.zip
 mv /tmp/wordpress /var/www/$fname
 clear
 echo "######################################################"
 printf "Open url $dn in your browser and install wordpress. \n read file /root/$fname for database information to be use during installation\n"
 printf " Need to setup another domain? please run vhostsetup command again! \n"
 echo "######################################################"
 cd
 rm -rf /tmp/mdbpassw /tmp/wordpress/ /tmp/latest.zip > /dev/null 2>&1
 chown -R www-data:www-data /var/www/$fname
 exit 0
;;

x|X)exit;;
*) echo "Wrong choice! Please select correct option menu!";;
esac
done
