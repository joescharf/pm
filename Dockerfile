FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
RUN addgroup -S pm && adduser -S pm -G pm
RUN mkdir -p /data && chown pm:pm /data
ARG TARGETPLATFORM
COPY ${TARGETPLATFORM}/pm /usr/local/bin/pm
USER pm
ENV PM_DB_PATH=/data/pm.db
EXPOSE 8080
ENTRYPOINT ["pm"]
CMD ["serve"]
