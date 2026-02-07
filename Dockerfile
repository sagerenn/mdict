FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS build

ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

WORKDIR /src

COPY go.mod go.sum ./
COPY vendor ./vendor
RUN if [ -d vendor ]; then \
      go env -w GOFLAGS=-mod=vendor; \
    else \
      go mod download; \
    fi

COPY . .
RUN CGO_ENABLED=0 \
    GOOS=$TARGETOS \
    GOARCH=$TARGETARCH \
    GOARM=${TARGETVARIANT#v} \
    go build -trimpath -ldflags "-s -w" -o /out/gdapi ./cmd/gdapi

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/gdapi /gdapi

EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/gdapi"]
