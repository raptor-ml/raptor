ARG BASE_PYTHON_IMAGE=python:3.12-slim-bookworm
ARG VERSION
ARG GRPC_HEALTH_PROBE_VERSION=v0.4.18

## Build protobuf files
FROM bufbuild/buf:1.29.0 as buf
WORKDIR /workspace
COPY ./api/proto /workspace
RUN buf generate --include-imports .

# GRPC Health Probe
FROM golang:1.21 AS grpc_health_probe
ARG GRPC_HEALTH_PROBE_VERSION
RUN GOBIN=/out/ CGO_ENABLED=0 go install github.com/grpc-ecosystem/grpc-health-probe@${GRPC_HEALTH_PROBE_VERSION}

## Runtime image

FROM ${BASE_PYTHON_IMAGE} as runtime
ARG VERSION

LABEL org.opencontainers.image.source="https://github.com/raptor-ml/raptor"
LABEL org.opencontainers.image.version="${VERSION}"
LABEL org.opencontainers.image.url="https://raptor.ml"
LABEL org.opencontainers.image.title="Raptor Runtime"
LABEL org.opencontainers.image.description="Raptor Runtime is a sidecar that provides tooling for Raptor extensions"

COPY --from=grpc_health_probe /out/grpc-health-probe /bin/grpc_health_probe
RUN chmod +x /bin/grpc_health_probe

RUN pip install --upgrade pip

RUN groupadd -g 65532 nonroot
RUN useradd -u 65532 -g nonroot -d /home/nonroot -m -s /bin/bash nonroot

WORKDIR /runtime
COPY ./runtime/requirements.txt /runtime/
ENV GRPC_PYTHON_BUILD_SYSTEM_OPENSSL=1
ENV GRPC_PYTHON_BUILD_SYSTEM_ZLIB=1
RUN pip install -r requirements.txt

COPY ./runtime /runtime
COPY --from=buf /workspace/gen/python /runtime/proto
COPY ./labsdk/raptor/program.py .

USER 65532:65532
ENV PYTHONUNBUFFERED=1
CMD [ "python", "./main.py" ]
