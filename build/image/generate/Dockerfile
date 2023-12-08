FROM golang:1.19

RUN go install github.com/daixiang0/gci@v0.3.3 && \
    go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.6.2 && \
    go clean -cache

ENV GOROOT=/usr/local/go
