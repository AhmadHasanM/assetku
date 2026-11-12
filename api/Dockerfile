# syntax=docker/dockerfile:1
# AssetKu API: static Go binary on distroless. Build context: ./api

FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY third_party ./third_party
RUN go mod download
COPY . .
# timetzdata embeds the zone database, so Asia/Jakarta never depends on the image.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -tags timetzdata -o /out/server ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/server /server
ENV ADDR=:8080 \
    TZ=Asia/Jakarta
EXPOSE 8080
USER nonroot:nonroot
# Subcommands: `/server create-superadmin ...`, `/server seed-demo ...`, `/server healthcheck`.
ENTRYPOINT ["/server"]
