ARG VERSION=dev
ARG BUILD_TIME=unknown
ARG GIT_COMMIT=unknown

FROM node:20-alpine AS frontend-builder
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.26-alpine AS backend-builder
ARG VERSION
ARG BUILD_TIME
ARG GIT_COMMIT
WORKDIR /app/server
RUN apk add --no-cache gcc musl-dev
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
RUN CGO_ENABLED=1 GOOS=linux go build \
	  -ldflags "-X smallgo/server/version.Version=${VERSION} -X smallgo/server/version.BuildTime=${BUILD_TIME} -X smallgo/server/version.GitCommit=${GIT_COMMIT} -X smallgo/server/version.AppName=提醒事项" \
	  -o /out/reminder .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
RUN adduser -D -u 1000 appuser
WORKDIR /app
COPY --from=backend-builder /out/reminder ./reminder
COPY --from=frontend-builder /app/web/dist ./static/dist
RUN mkdir -p /app/data && chown -R appuser:appuser /app
USER appuser
EXPOSE 8906
VOLUME ["/app/data"]
HEALTHCHECK --interval=30s --timeout=3s --retries=3 \
  CMD wget -qO- http://localhost:8906/api/version || exit 1
ENTRYPOINT ["./reminder"]
CMD ["-data-dir", "/app/data", "-web-dir", "./static/dist"]
