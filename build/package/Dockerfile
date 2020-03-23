FROM ubuntu:focal

LABEL app_name="autowp.traffic"
LABEL maintainer="dmitry@pereslegin.ru"

ENV TRAFFIC_INPUT_HOST "localhost"
ENV TRAFFIC_INPUT_PORT "5672"
ENV TRAFFIC_INPUT_QUEUE "input"
ENV TRAFFIC_MYSQL_DSN "root:password@tcp(localhost:3306)/traffic"
ENV TRAFFIC_MIGRATIONS_DSN "mysql://root:password@tcp(localhost)/traffic"
ENV TRAFFIC_SENTRY_DSN ""
ENV TRAFFIC_SENTRY_ENVIRONMENT "production"

VOLUME /var/log
  
WORKDIR /app

ENTRYPOINT ["sh", "/entrypoint.sh"]

COPY entrypoint.sh /entrypoint.sh

RUN DEBIAN_FRONTEND=noninteractive apt-get update -qq -y && \
    DEBIAN_FRONTEND=noninteractive apt-get dist-upgrade -qq -y && \
    DEBIAN_FRONTEND=noninteractive apt-get install -qq -y \
        ca-certificates \
        wget && \
    DEBIAN_FRONTEND=noninteractive apt-get autoremove -y && \
    DEBIAN_FRONTEND=noninteractive apt-get autoclean -y && \
    \
    wget -q -O /usr/local/bin/waitforit https://github.com/maxcnunes/waitforit/releases/download/v2.4.1/waitforit-linux_amd64 && \
    chmod +x /usr/local/bin/waitforit
 

COPY traffic /app/traffic

COPY migrations /app/migrations
