FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /markdown-webdav-backend .

FROM alpine:3.22
# git does the versioning; the server refuses to start without it.
RUN apk add --no-cache git ca-certificates && adduser -D -u 1000 vault \
    && mkdir /vault && chown vault /vault
COPY --from=build /markdown-webdav-backend /usr/local/bin/
USER vault
ENV MDWEBDAV_VAULT=notes=/vault MDWEBDAV_LISTEN=:8080
VOLUME /vault
EXPOSE 8080
ENTRYPOINT ["markdown-webdav-backend"]
