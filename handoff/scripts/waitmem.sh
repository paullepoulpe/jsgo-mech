#!/bin/bash
# waits until at least ${1:-6} GB of memory are available (a Go compile of the package needs ~4.5 GB)
need=$(( ${1:-6} * 1024 * 1024 ))
for i in $(seq 1 120); do
  a=$(awk '/MemAvailable/ {print $2}' /proc/meminfo)
  [ "$a" -ge "$need" ] && exit 0
  sleep 5
done
exit 0
