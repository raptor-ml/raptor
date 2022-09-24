### Build
FROM golang:1.18 AS build
ARG TARGETOS
ARG TARGETARCH
ARG LDFLAGS
ARG VERSION

WORKDIR /workspace
COPY go.mod /workspace
COPY go.sum /workspace
RUN go mod download
COPY . /workspace

### Core
FROM build AS build-core
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -ldflags="${LDFLAGS}" -o /out/core cmd/core/*.go

FROM gcr.io/distroless/static:nonroot as core

LABEL org.opencontainers.image.source="https://github.com/raptor-ml/raptor"
LABEL org.opencontainers.image.version="${VERSION}"
LABEL org.opencontainers.image.url="https://raptor.ml"
LABEL org.opencontainers.image.title="Raptor Core"
LABEL org.opencontainers.image.description="Raptor Core is the extension that implements on Kubernetes"

WORKDIR /
COPY --from=build-core /out/core .
USER 65532:65532

ENTRYPOINT ["/core"]

### Historian
FROM build AS build-historian
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -ldflags="${LDFLAGS}" -o /out/historian cmd/historian/*.go

FROM gcr.io/distroless/static:nonroot as historian

LABEL org.opencontainers.image.source="https://github.com/raptor-ml/raptor"
LABEL org.opencontainers.image.version="${VERSION}"
LABEL org.opencontainers.image.url="https://raptor.ml"
LABEL org.opencontainers.image.title="Raptor Historian"
LABEL org.opencontainers.image.description="Raptor Historian is responsible to record the historical data of the production results"

WORKDIR /
COPY --from=build-historian /out/historian .
USER 65532:65532

ENTRYPOINT ["/historian"]

### Runtime sidecar
FROM build AS build-runtime
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -ldflags="${LDFLAGS}" -o /out/runtime cmd/runtime/*.go

FROM gcr.io/distroless/static:nonroot as runtime

LABEL org.opencontainers.image.source="https://github.com/raptor-ml/raptor"
LABEL org.opencontainers.image.version="${VERSION}"
LABEL org.opencontainers.image.url="https://raptor.ml"
LABEL org.opencontainers.image.title="Raptor Runtime"
LABEL org.opencontainers.image.description="Raptor Runtime is a sidecar that provides tooling for Raptor extensions"

WORKDIR /
COPY --from=build-runtime /out/runtime .
USER 65532:65532

ENTRYPOINT ["/runtime"]
