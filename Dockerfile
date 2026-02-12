FROM alpine:latest
RUN apk add --no-cache ca-certificates
ARG TARGETPLATFORM
COPY $TARGETPLATFORM/pm /usr/local/bin/pm
ENTRYPOINT ["pm"]
