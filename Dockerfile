FROM golang:1.24.6-alpine3.22 AS builder

# Set necessary environmet variables needed for our image
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# Move to working directory /build
WORKDIR /build

# Copy and download dependency using go mod
COPY go.mod .
COPY go.sum .
RUN go mod download

# Copy the code into the container
COPY . .

# Build the application
RUN go build -o main .

WORKDIR /dist

# Copy binary from build to main folder
RUN cp /build/main .

# Build a small image
FROM alpine:3.22

COPY --from=builder /dist/ /runner

RUN apk add --no-cache git
RUN apk --update add openssh-client

# RUN mkdir /root/.ssh

# RUN chown 65532 /etc/ssh/ssh_config 
# RUN chown -R 65532 /root/.ssh && chmod -R go-rwx /root/.ssh

# USER 65532:65532

ENTRYPOINT ["/runner/main"]