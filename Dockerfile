FROM golang:1.25 AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /cloud-controller-manager ./cmd/cloud-controller-manager

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /cloud-controller-manager /cloud-controller-manager
ENTRYPOINT ["/cloud-controller-manager"]
