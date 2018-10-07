mkdir -p /var/log/supervisor

waitforit -address tcp://$TRAFFIC_MYSQL_HOST:$TRAFFIC_MYSQL_PORT -timeout 30
waitforit -address tcp://$TRAFFIC_INPUT_HOST:$TRAFFIC_INPUT_PORT -timeout 30

/usr/bin/supervisord -c /etc/supervisor/supervisord.conf
