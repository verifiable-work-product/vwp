# Multi-stage build for the vwp-api Cloud Run service.
# BUILD CONTEXT MUST BE THE REPO ROOT, because backend/ uses a go.mod `replace`
# pointing at ../sdk/go:
#   docker build -f backend/Dockerfile -t vwp-api .
FROM golang:1.24-alpine AS build
WORKDIR /src

# Copy the two modules the build needs: the backend and the SDK it replaces.
COPY sdk/go/ ./sdk/go/
COPY backend/ ./backend/

WORKDIR /src/backend
RUN go mod download
# Static, stripped binary; embeds examples/*.vwp via go:embed.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /vwp-api .

# Minimal runtime. distroless static: no shell, no package manager, runs nonroot.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /vwp-api /vwp-api
# Cloud Run sets $PORT (default 8080); the server reads it.
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/vwp-api"]
