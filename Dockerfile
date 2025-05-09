# First stage: Build the Go binary
FROM golang:1.24.2 AS builder

WORKDIR /event-queue

ADD ./ /event-queue

RUN make build/api

# Second stage: Create a minimal runtime image
FROM debian:bookworm-slim

WORKDIR /

# Copy built binaries from the builder stage
COPY --from=builder /event-queue/bin/behavox-local-compatible /event-queue/behavox-event-queue

# Expose the necessary ports
EXPOSE 80
EXPOSE 443

# Set the entrypoint for the container
ENTRYPOINT ["/event-queue/behavox-event-queue"]