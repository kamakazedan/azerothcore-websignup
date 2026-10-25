# Build: one static binary with everything in it (page, styles, fonts, art).
FROM golang:1-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/websignup .

# Run: no shell, no package manager, not root.
FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/websignup /websignup
USER nonroot:nonroot
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 CMD ["/websignup", "-healthcheck"]
ENTRYPOINT ["/websignup"]
