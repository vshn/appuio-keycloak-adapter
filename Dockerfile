FROM docker.io/library/alpine:3.19 as runtime

RUN \
  apk add --update --no-cache \
    bash \
    curl \
    ca-certificates \
    tzdata

ENTRYPOINT ["appuio-keycloak-adapter"]
COPY appuio-keycloak-adapter /usr/bin/

USER 65536:0
