FROM golang:1.27-alpine AS build

WORKDIR /src

ENV CGO_ENABLED=0 \
    GOOS=linux

COPY go.mod go.su[m] ./
RUN go mod download

COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM gcr.io/distroless/static:nonroot

COPY --from=build /out/server /server

USER nonroot:nonroot
EXPOSE 4000

ENTRYPOINT ["/server"]
