FROM gcr.io/distroless/static:nonroot as historian
ARG APP
WORKDIR /
COPY historian .
USER 65532:65532

ENTRYPOINT ["/historian"]

FROM gcr.io/distroless/static:nonroot as manager
WORKDIR /
COPY manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
