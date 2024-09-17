FROM --platform=${BUILDPLATFORM:-linux/amd64} golang:1.23 AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download

COPY main.go main.go

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -a -o coredns-reload main.go

FROM --platform=${TARGETPLATFORM:-linux/amd64} busybox:1.36
WORKDIR /
COPY --from=builder --chown=root:root /app/coredns-reload .

ENTRYPOINT ["/coredns-reload"]
