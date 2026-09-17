FROM alpine:3.24
RUN apk add --no-cache curl=8.22.0-r0
CMD ["sh"]
