# syntax=docker/dockerfile:1

# ---- build stage ----------------------------------------------------------
FROM golang:1.25-alpine AS build
WORKDIR /src
# go.mod pins the exact Go patch release; let the toolchain fetch it if the
# image ships an older one.
ENV GOTOOLCHAIN=auto
# Dependencies first, so the layer is cached while the source changes.
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/api ./cmd/api \
 && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/bootstrap ./cmd/bootstrap

# ---- runtime stage --------------------------------------------------------
FROM alpine:3.21
RUN adduser -D -H app
WORKDIR /app
# No migrations directory and no web/ directory: both binaries embed the SQL files,
# the templates and the static assets (Lesson 10).
COPY --from=build /out/api /out/bootstrap /app/
USER app
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/app/api"]
