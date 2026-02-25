package export_templates

import "fmt"

// DockerfileForStack returns a Dockerfile appropriate for the detected tech stack
func DockerfileForStack(stack string) string {
	switch stack {
	case "node", "react", "vue", "next", "svelte":
		return `FROM node:20-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM node:20-alpine
WORKDIR /app
COPY --from=builder /app/dist ./dist
COPY --from=builder /app/node_modules ./node_modules
COPY --from=builder /app/package.json .
EXPOSE 3000
CMD ["node", "dist/index.js"]
`
	case "go", "golang":
		return `FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /app/server ./cmd/...

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/server /server
EXPOSE 8080
CMD ["/server"]
`
	case "python", "django", "flask", "fastapi":
		return `FROM python:3.12-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
EXPOSE 8000
CMD ["python", "-m", "uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8000"]
`
	case "rust":
		return `FROM rust:1.75-slim AS builder
WORKDIR /app
COPY Cargo.toml Cargo.lock ./
COPY src/ ./src/
RUN cargo build --release

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=builder /app/target/release/app /app
EXPOSE 8080
CMD ["/app"]
`
	default:
		return fmt.Sprintf("# Dockerfile for %s\n# TODO: Add appropriate base image and build steps\nFROM alpine:3.19\nWORKDIR /app\nCOPY . .\nEXPOSE 8080\n", stack)
	}
}
