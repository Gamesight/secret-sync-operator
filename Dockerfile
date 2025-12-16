# Build the manager binary
FROM golang:1.23 AS builder

WORKDIR /workspace

# Allow Go to download newer toolchain if required by dependencies
ENV GOTOOLCHAIN=auto

# Copy go mod files
COPY go.mod go.mod
COPY go.sum go.sum

# Cache deps before building and copying source
RUN go mod download

# Copy the go source
COPY cmd/ cmd/
COPY pkg/ pkg/

# Build
# CGO_ENABLED=0 for static binary
# GOOS=linux for Linux target
ARG TARGETOS=linux
ARG TARGETARCH=amd64

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -a -installsuffix cgo \
    -ldflags="-w -s" \
    -o manager ./cmd/manager

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot

WORKDIR /

# Copy the binary from builder
COPY --from=builder /workspace/manager .

# Use nonroot user (UID 65532)
USER 65532:65532

ENTRYPOINT ["/manager"]
