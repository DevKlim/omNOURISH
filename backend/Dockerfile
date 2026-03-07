FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY . .

# Delete the bad local go.sum that was just copied, and securely regenerate it
RUN rm -f go.sum && go mod tidy

# Build the app using the fresh dependencies
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/main .

EXPOSE 8080
CMD ["./main"]
