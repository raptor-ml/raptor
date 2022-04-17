### Historian
FROM gcr.io/distroless/static:nonroot as historian
ARG APP
WORKDIR /
COPY historian .
USER 65532:65532

ENTRYPOINT ["/historian"]

### Core
FROM gcr.io/distroless/static:nonroot as core
WORKDIR /
COPY core .
USER 65532:65532

ENTRYPOINT ["/core"]

### Runtime sidecar
FROM gcr.io/distroless/static:nonroot as runtime
WORKDIR /
COPY runtime .
USER 65532:65532

ENTRYPOINT ["/runtime"]
