# Use the official Golang image to build the application
FROM golang:1.24 AS builder

# Set necessary environment variables
ENV GO111MODULE=on

# Set the working directory
WORKDIR /app

# Copy the go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the application
# Disable CGO for a statically linked, smaller binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o app cmd/app/main.go

# --- Deployment Stage ---
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Set the working directory
WORKDIR /root/

# Copy the built binary from the builder stage
COPY --from=builder /app/app .
COPY --from=builder /app/.env .
COPY --from=builder /app/internal/db/migrations/data ./internal/db/migrations/data

# Set the entry point to run the application
ENTRYPOINT ["./app"]