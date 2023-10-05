# Always use explicit golang and base OS version for the image
FROM 274295908850.dkr.ecr.eu-west-1.amazonaws.com/i/golang:1.21.1-alpine3.18

# Switch to generic build directory
WORKDIR /usr/src/service

# Download and verify dependencies
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy in source files
COPY main.go main.go

# Build the service
RUN go build -v -o /usr/local/bin/service ./...

ENTRYPOINT ["service"]
