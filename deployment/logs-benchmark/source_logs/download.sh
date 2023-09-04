#!/bin/bash

set -ex

# Unarchived size: 5.1M Apache.log
if [ ! -f Apache.tar.gz ]; then
  curl -o Apache.tar.gz -C - https://zenodo.org/record/3227177/files/Apache.tar.gz?download=1
fi

# Unarchived size: 13G hadoop-*.log
if [ ! -f HDFS_2.tar.gz ]; then
  curl -o HDFS_2.tar.gz -C - https://zenodo.org/record/3227177/files/HDFS_2.tar.gz?download=1
fi

# Unarchived size: 2.3M Linux.log
if [ ! -f Linux.tar.gz ]; then
  curl -o Linux.tar.gz -C - https://zenodo.org/record/3227177/files/Linux.tar.gz?download=1
fi

# Unarchived size: 32G Thunderbird.log
if [ ! -f Thunderbird.tar.gz ]; then
  curl -o Thunderbird.tar.gz -C - https://zenodo.org/record/3227177/files/Thunderbird.tar.gz?download=1
fi

# Unarchived size: 73M SSH.log
if [ ! -f SSH.tar.gz ]; then
  curl -o SSH.tar.gz -C - https://zenodo.org/record/3227177/files/SSH.tar.gz?download=1
fi

mkdir -p logs

for file in *.tar.gz; do
  tar -xzf $file -C logs
done
