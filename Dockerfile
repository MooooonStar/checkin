FROM alpine:3.7

RUN apk update \
        && apk upgrade \
        && apk add --no-cache \
        ca-certificates \
        && update-ca-certificates 2>/dev/null || true

RUN mkdir -p /usr/local/checkin

COPY config.yml /usr/local/checkin

WORKDIR /usr/local/checkin

ADD ./checkin /usr/local/checkin

RUN chmod +x checkin

ENTRYPOINT ["./checkin"]