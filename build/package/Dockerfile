FROM ubuntu

LABEL app_name="autowp.traffic"
LABEL maintainer="dmitry@pereslegin.ru"

VOLUME /var/log

WORKDIR /app

RUN DEBIAN_FRONTEND=noninteractive apt-get update -qq -y && \
    DEBIAN_FRONTEND=noninteractive apt-get dist-upgrade -qq -y && \
    DEBIAN_FRONTEND=noninteractive apt-get install -qq -y \
        ca-certificates \
        supervisor && \
    DEBIAN_FRONTEND=noninteractive apt-get autoremove -y && \
    DEBIAN_FRONTEND=noninteractive apt-get autoclean -y
 
COPY etc /etc
 
COPY traffic /app/traffic
 
COPY entrypoint.sh /entrypoint.sh

ENTRYPOINT ["sh", "/entrypoint.sh"]
