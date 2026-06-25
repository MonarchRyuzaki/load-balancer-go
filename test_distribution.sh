#!/bin/bash

echo "Testing Load Balancer Distribution..."
echo "Sending 100 requests from 100 different loopback IPs (127.0.1.1 to 127.0.1.100)"
echo "------------------------------------------------------"

# Loop 100 times, using a different source IP each time
for i in {1..100}; do
  # -w 1: Timeout after 1 second so it doesn't hang
  # -s 127.0.1.$i: Spoof the source IP
  # grep -o: Only extract the "port 808x" part of the greeting
  echo "" | nc -w 1 -s 127.0.1.$i 127.0.0.1 8080 2>/dev/null | grep -o "port [0-9]\+"
done | sort | uniq -c

echo "------------------------------------------------------"
