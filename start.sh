#!/bin/bash
cd /opt/netgazer
exec bin/server \
  --grpc-port=50051 \
  --http-port=8080 \
  --db=/opt/netgazer/netgazer.db \
  --web-dir=/opt/netgazer/frontend/dist \
  >> /var/log/netgazer.log 2>&1
