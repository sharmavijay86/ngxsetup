# Nginx setup on Ubuntu
Installation script to compile nginx with tune up settings clone the script in /root/ dir run bash ngxsetup.sh Follow instruction ...
## Securing php-fpm pool
```
cd /etc/php/7.4/fpm/pool.d
cp www.conf web1.conf
vim web1.conf
# to change here
[web1]
user = web1
group = web1
listen = /run/php/php7.4-fpm-web1.sock
```
change nginx
```
vim /etc/nginx/sites-enabled/web1lan
fastcgi_pass unix:/run/php/php7.4-fpm-web1.sock;
```
restrt
```
systemctl restart nginx
systemctl restart php7.4-fpm
```
Done..
