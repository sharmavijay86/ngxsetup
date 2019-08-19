#!/bin/bash
HOSTNAME=`/bin/uname -n`
OS_TYPE=`/bin/uname -s | sed 's/\n//g'`
OS_VERSION=`lsb_release -a | grep -i Description | awk -F " " '{print $2":"$3}'`
KERNEL=`/bin/uname -r`
ARCH=`/bin/uname -m`
CPUS_SPEED=`grep MHz /proc/cpuinfo | tail -1 | awk '{ print $4 }' | cut -d. -f1`
MEMORY_SERVER=`free -m | head -n 2 | tail -1 | awk '{ print $2"MB" }'`
BOOT=`who -b | awk '{ printf $3" "$4" "$5 }'`
SERVERTYP_SERVER=`dmidecode | grep "Product Name" | head -1 | cut -d" " -f3,4,5`
MKDIR=/bin/mkdir
COPY=/bin/cp
INFRA_DIR=/var/infra/
COMMON_FILE=$INFRA_DIR/common
SERVER_INFO_FILE=$INFRA_DIR/"$HOSTNAME".xml
if [ ! -d $INFRA_DIR ]
then
        $MKDIR -p $INFRA_DIR
fi
CPUSSW="false"
CPUS=`grep processor /proc/cpuinfo | wc -l`
grep "cpu cores" /proc/cpuinfo >/dev/null 2>&1
if [ $? -eq 0 ]; then
   CPUS_CORE=`grep "cpu cores" /proc/cpuinfo  | tail -1 | awk '{ print $4 }'`
   else
     CPUSSW="true"
fi
grep "flags.* ht .*" /proc/cpuinfo >/dev/null 2>&1
if [ $? -eq 0 ]; then
   if [ $CPUSSW == "true" ]; then
      CPUS="$CPUS Processor[s]"
      else
          if [ $CPUS -eq 1 ]; then
             CPUS="$CPUS Processor[s]"
             else
                 CPUS_HT=$((CPUS/$CPUS_CORE))
                 CPUS="$CPUS_HT x processor[s] with $CPUS_CORE core"
          fi
   fi
   else
       CPUS="$CPUS Processor[s]"
fi
