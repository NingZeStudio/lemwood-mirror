FROM node:20-alpine AS frontend-builder
WORKDIR /src
COPY web/package.json web/pnpm-lock.yaml ./web/
COPY admin-app/package.json admin-app/pnpm-lock.yaml ./admin-app/
RUN corepack enable && cd web && pnpm install --frozen-lockfile
RUN cd admin-app && pnpm install --frozen-lockfile
COPY web/ ./web/
COPY admin-app/ ./admin-app/
RUN cd web && pnpm build
RUN cd admin-app && pnpm build

FROM golang:1.24-alpine AS go-builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend-builder /src/web/dist ./web/dist
COPY --from=frontend-builder /src/web/admin ./web/admin
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o /mirror ./cmd/mirror

FROM alpine:3.21
RUN apk add --no-cache ca-certificates git
COPY --from=go-builder /mirror /mirror
VOLUME ["/data"]
WORKDIR /data
EXPOSE 8080
ENTRYPOINT ["/mirror"]
