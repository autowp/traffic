mkdir -p /var/log/supervisor

waitforit -address tcp://$TRAFFIC_RABBITMQ_HOST:$TRAFFIC_RABBITMQ_PORT -timeout 30

/usr/bin/supervisord -c /etc/supervisor/supervisord.conf
