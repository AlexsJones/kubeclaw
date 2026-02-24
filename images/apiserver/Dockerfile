# API Server
FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git ca-certificates
WORKDIR /workspace
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /apiserver ./cmd/apiserver

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /apiserver /apiserver
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/apiserver"]
