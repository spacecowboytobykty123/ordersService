# Stage 1: Build the Go binary
FROM golang:1.24.1 AS builder

WORKDIR /app

# Copy go.mod and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire source code
COPY . .

# Set the entrypoint path (adjust if needed)
WORKDIR /app/cmd/api

# Build the main entry point
RUN CGO_ENABLED=0 GOOS=linux go build -o /server

# Stage 2: Run it in a minimal runtime image
FROM gcr.io/distroless/base-debian11

WORKDIR /app
COPY --from=builder /server .

# Expose ports
EXPOSE 4000 5050

ENTRYPOINT ["/app/server"]
