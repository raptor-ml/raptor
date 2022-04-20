### Historian
FROM gcr.io/distroless/static:nonroot as historian
ARG APP
WORKDIR /
COPY bin/historian .
USER 65532:65532

ENTRYPOINT ["/historian"]

### Core
FROM gcr.io/distroless/static:nonroot as core
WORKDIR /
COPY bin/core .
USER 65532:65532

ENTRYPOINT ["/core"]

### Runtime sidecar
FROM gcr.io/distroless/static:nonroot as runtime
WORKDIR /
COPY bin/runtime .
USER 65532:65532

ENTRYPOINT ["/runtime"]
