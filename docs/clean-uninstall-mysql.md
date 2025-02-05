```bash
  sudo systemctl stop mysql
  sudo apt-get purge mysql-server mysql-client mysql-common
  sudo apt-get autoremove
  sudo rm -rf /etc/mysql
  sudo rm -rf /var/lib/mysql
  sudo apt-get remove --purge mysql*
  sudo apt-get autoclean
  which mysql
  mysql --version
```
