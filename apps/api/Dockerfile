# syntax=docker/dockerfile:1

FROM golang:1.26 AS build
WORKDIR /src
# goproxy.cn keeps `go mod download` working from mainland China; `,direct`
# falls back to the origin elsewhere, so this stays correct outside China too.
ENV GOPROXY=https://goproxy.cn,direct
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /api ./cmd/api

FROM gcr.io/distroless/static:nonroot
COPY --from=build /api /api
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/api"]
