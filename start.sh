#!/bin/bash
cd /root/gtopng
exec bin/server --grpc-port=50051 --http-port=8080 --db=/root/gtopng/gtopng.db >> /var/log/gtopng.log 2>&1
