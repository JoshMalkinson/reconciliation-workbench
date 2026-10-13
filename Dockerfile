FROM node:24-bookworm-slim AS web
WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.27.1-bookworm AS backend
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
COPY gen/ gen/
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /reconcile ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=backend /reconcile /app/reconcile
COPY --from=web /src/web/dist /app/web/dist
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/app/reconcile"]
