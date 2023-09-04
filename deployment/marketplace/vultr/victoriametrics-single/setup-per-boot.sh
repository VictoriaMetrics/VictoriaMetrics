#!/bin/bash

## Run on every boot.
echo $(date -u) ": System booted." >> /var/log/per-boot.log