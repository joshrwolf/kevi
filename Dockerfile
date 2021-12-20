# Build the kevi binary
FROM golang:1.16 as builder

WORKDIR /workspace
COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download

COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/
COPY pkg/ pkg/
COPY cli/ cli/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o kevi main.go

# Use distroless as minimal base image to package the kevi binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
#FROM scratch
WORKDIR /
COPY --from=builder /workspace/kevi .
USER 65532:65532

ENTRYPOINT ["/kevi", "manager"]
