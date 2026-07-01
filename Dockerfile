# --- build stage ---
FROM golang:1.25-alpine AS build
WORKDIR /src

# Cache deps first.
COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /bin/server ./cmd/server

# --- run stage ---
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /bin/server /bin/server
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/bin/server"]
