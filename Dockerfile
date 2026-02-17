# Stage 1: Build Angular web UI
FROM node:22-alpine AS web-builder
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/angular.json web/tsconfig.json web/tsconfig.app.json ./
COPY web/src ./src
RUN npm run build

# Stage 2: Build Go binary
FROM golang:1.25 AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
# Cache deps before building and copying source
COPY go.mod go.sum ./
RUN go mod download

# Copy the Go source
COPY cmd/ cmd/
COPY api/ api/
COPY internal/ internal/

# Build
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o manager cmd/main.go

# Stage 3: Final image
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager .
COPY --from=web-builder /web/dist/web/browser /web/dist/web/browser
USER 65532:65532

ENTRYPOINT ["/manager"]
