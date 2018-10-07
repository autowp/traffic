#!/bin/sh

./waitforit -address tcp://$TRAFFIC_MYSQL_HOST:$TRAFFIC_MYSQL_PORT -timeout 30

maxcounter=45
 
counter=1
while ! mysql --protocol=tcp --host=$TRAFFIC_MYSQL_HOST --port=$TRAFFIC_MYSQL_PORT --user=$TRAFFIC_MYSQL_USERNAME -p$TRAFFIC_MYSQL_PASSWORD -e "show databases;" > /dev/null 2>&1; do
    printf "."
    sleep 1
    counter=`expr $counter + 1`
    if [ $counter -gt $maxcounter ]; then
        >&2 echo "We have been waiting for MySQL too long already; failing."
        exit 1
    fi;
done

echo -e "\nmysql ready"