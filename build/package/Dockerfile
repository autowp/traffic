FROM ubuntu

LABEL app_name="autowp.traffic"
LABEL maintainer="dmitry@pereslegin.ru"

ENV TRAFFIC_INPUT_HOST "localhost"
ENV TRAFFIC_INPUT_PORT "5672"
ENV TRAFFIC_INPUT_QUEUE "input"
ENV TRAFFIC_MYSQL_HOST "localhost"
ENV TRAFFIC_MYSQL_PORT "3306"
ENV TRAFFIC_MYSQL_USERNAME "root"
ENV TRAFFIC_MYSQL_PASSWORD "root"
ENV TRAFFIC_MYSQL_DBNAME "traffic"
ENV TRAFFIC_ROLLBAR_TOKEN ""
ENV TRAFFIC_ROLLBAR_ENVIRONMENT "production"
ENV TRAFFIC_ROLLBAR_PERIOD "1m"

VOLUME /var/log
  
WORKDIR /app

RUN DEBIAN_FRONTEND=noninteractive apt-get update -qq -y && \
    DEBIAN_FRONTEND=noninteractive apt-get dist-upgrade -qq -y && \
    DEBIAN_FRONTEND=noninteractive apt-get install -qq -y \
        ca-certificates \
        supervisor \
        wget && \
    DEBIAN_FRONTEND=noninteractive apt-get autoremove -y && \
    DEBIAN_FRONTEND=noninteractive apt-get autoclean -y && \
    \
    wget -q -O /usr/local/bin/waitforit https://github.com/maxcnunes/waitforit/releases/download/v2.4.0/waitforit-linux_amd64 && \
    chmod +x /usr/local/bin/waitforit
 
COPY etc /etc
 
COPY traffic /app/traffic
 
COPY entrypoint.sh /entrypoint.sh

ENTRYPOINT ["sh", "/entrypoint.sh"]
