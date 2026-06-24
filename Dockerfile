FROM node:20-alpine AS frontend-builder
WORKDIR /src
COPY frontend/package.json frontend/pnpm-lock.yaml ./frontend/
COPY admin-app/package.json admin-app/pnpm-lock.yaml ./admin-app/
RUN corepack enable && cd frontend && pnpm install --frozen-lockfile
RUN cd admin-app && pnpm install --frozen-lockfile
COPY frontend/ ./frontend/
COPY admin-app/ ./admin-app/
RUN cd frontend && pnpm build
RUN cd admin-app && pnpm build

FROM golang:1.24-alpine AS go-builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend-builder /src/web/default ./web/default
COPY --from=frontend-builder /src/web/admin ./web/admin
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o /mirror ./cmd/mirror

FROM alpine:3.21
RUN apk add --no-cache ca-certificates git
COPY --from=go-builder /mirror /mirror
VOLUME ["/data"]
WORKDIR /data
EXPOSE 8080
ENTRYPOINT ["/mirror"]
