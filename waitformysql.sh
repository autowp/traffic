#!/bin/bash

./waitforit -address tcp://$TRAFFIC_MYSQL_HOST:$TRAFFIC_MYSQL_PORT -timeout 30

echo "Waiting for mysql"
until mysql -h"$TRAFFIC_MYSQL_HOST" -P"$TRAFFIC_MYSQL_PORT" -u$TRAFFIC_MYSQL_USERNAME -p"$TRAFFIC_MYSQL_PASSWORD" &> /dev/null
do
  printf "."
  sleep 1
done

echo -e "\nmysql ready"