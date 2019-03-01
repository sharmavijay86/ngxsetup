#!/bin/bash
NAME=$(date +%d%b%Y)
ncp=$(nproc)
CLOAD=$(uptime | awk '{print $10}' | cut -d'.' -f1)
if [ $CLOAD -gt $ncp ];then
echo "$NAME : current load is $CLOAD hence service down" >> /home/load.log
service nginx stop
else
echo "Server Load is ok! "
fi


SRVSTAT=$(ps -ef | grep -v grep | grep nginx | wc -l)
if  [ $SRVSTAT -eq '0' ] && [ $CLOAD -lt $ncp ]
then
service nginx start
else
echo "nginx is running!!!"
fi
