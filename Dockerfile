FROM golang:1.22.6 AS builder

# Turn on Go modules support and disable CGO
ENV GO111MODULE=on CGO_ENABLED=0

WORKDIR /src

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Compile the action
RUN go build \
    -a \
    -trimpath \
    -ldflags "-s -w -extldflags '-static'" \
    -installsuffix cgo \
    -tags netgo \
    -o /bin/action \
    . && \
    strip /bin/action

FROM scratch

# Copy over SSL certificates from the first step
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy over the compiled action from the first step
COPY --from=builder /bin/action /bin/action

# Specify the container's entrypoint as the action
ENTRYPOINT ["/bin/action"]
