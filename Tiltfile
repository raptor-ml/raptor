load('ext://helm_resource', 'helm_resource', 'helm_repo')
load('ext://cert_manager', 'deploy_cert_manager')
load('ext://restart_process', 'docker_build_with_restart')

deploy_cert_manager()

k8s_custom_deploy(
    "ingress-nginx",
    "kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml -o yaml",
    "kubectl delete -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml",
    deps=[],
)
k8s_resource(
    "ingress-nginx",
    port_forwards=["80:80", "443:443"],
    labels=["cluster-services"]
)

helm_repo("ot-helm", "https://ot-container-kit.github.io/helm-charts/", labels=["cluster-services"])
helm_resource(
    "redis-operator",
    "ot-helm/redis-operator",
    namespace="ot-operators",
    flags=["--create-namespace"],
    labels=["cluster-services"],
    resource_deps=["ot-helm"],
)

# Install prometheus-operator
helm_repo('prometheus-community', 'https://prometheus-community.github.io/helm-charts', labels=["cluster-services"])
helm_resource(
    'prometheus',
    'prometheus-community/kube-prometheus-stack',
    namespace='monitoring',
    labels=["cluster-services"],
    flags=[  ## Remove some components to make prometheus-operator lighter
        '--set', 'kubeApiServer.enabled=false',
        '--set', 'kubeEtcd.enabled=false',
        '--set', 'kubeControllerManager.enabled=false',
        '--set', 'kubeScheduler.enabled=false',
        '--set', 'alertmanager.enabled=false',
        '--set', 'kubeProxy.enabled=false',
        '--set', 'prometheus.prometheusSpec.retention=1d',
        '--create-namespace'
    ],
    resource_deps=['prometheus-community'],
)

k8s_yaml("./hack/dev/config/redis.yaml")
k8s_resource(
    objects=['redis-standalone:Redis'],
    new_name='redis',
    port_forwards=["6379:6379"],
    labels=["redis", "cluster-services"],
    resource_deps=["redis-operator"],
)


### Raptor

def patch_resources(yaml):
    resources = decode_yaml_stream(yaml)
    for resource in resources:
        if resource['kind'] == 'Deployment':
            if resource['metadata']['name'] == 'raptor-controller-core':
                resource['spec']['replicas'] = 1

            resource["spec"]["template"]["spec"]["securityContext"] = {}
            for container in resource['spec']['template']['spec']['containers']:
                container["securityContext"] = {}

    return encode_yaml_stream(resources)


#### CRD
k8s_yaml(kustomize('config/crd'), allow_duplicates=True)

base_ignores = ['Tiltfile', './hack', './bin', './config', './.git', './.github', '*.md', './lsp', '.venv']
go_ignores = base_ignores + ['./labsdk', './runtime']
py_ignores = base_ignores + ['./cmd', './internal', './pkg']

#### Runtime
docker_build_with_restart(
    'raptor-runtime',
    '.',
    dockerfile='runtime/Dockerfile',
    platform='linux/amd64',
    live_update=[
        sync('runtime/', '/app'),
        run('cd /app && pip install -r requirements.txt', trigger='./requirements.txt'),
    ],
    entrypoint=["python", "/runtime/runtime.py"],
    trigger=['runtime/'],
    ignore=py_ignores,
)

#### Controllers
base_dockerfile = """
FROM golang:1.19 AS build
RUN go install github.com/go-delve/delve/cmd/dlv@latest

WORKDIR /workspace
COPY go.mod /workspace
COPY go.sum /workspace
RUN go mod download

COPY . /workspace

"""

compile_cmd = """CGO_ENABLED=0 go build -gcflags "all=-N -l" -o /opt/{app} cmd/{app}/*.go"""
dlv_cmd = [
    '/go/bin/dlv',
    '--listen=0.0.0.0:2345',
    '--api-version=2',
    '--headless=true',
    '--only-same-user=false',
    '--accept-multiclient',
    '--check-go-version=false',
    "--log",
    '--check-go-version=false',
    'exec',
    '--continue',
    '--',
]

##### Core
docker_build_with_restart(
    "raptor-core",
    '.',
    dockerfile_contents=base_dockerfile + "RUN " + compile_cmd.format(app="core"),
    entrypoint=dlv_cmd + [
        '/opt/core',
        "--health-probe-bind-address=:8081",
        "--metrics-bind-address=127.0.0.1:8080",
        "--leader-elect",
        "-r=redis-standalone.default:6379",
    ],
    live_update=[
        # Copy the binary so it gets restarted.
        sync(
            ".", "/workspace"
        ),
        run(compile_cmd.format(app="core")),
        run('go mod download', trigger=['.go.mod', '.go.sum']),
    ],
    ignore=go_ignores,
)

##### Historian
docker_build_with_restart(
    "raptor-historian",
    '.',
    dockerfile_contents=base_dockerfile + "RUN " + compile_cmd.format(app="historian"),
    entrypoint=dlv_cmd + ['/opt/historian', "-r=redis-standalone.default:6379"],
    live_update=[
        # Copy the binary so it gets restarted.
        sync(
            ".", "/workspace"
        ),
        run(compile_cmd.format(app="historian"), trigger=[
            'cmd/historian/**',
            'internal/historian/*',
            'internal/plugins/providers/historical/**',
            'pkg/querybuilder/**'
        ]),
        run('go mod download', trigger=['.go.mod', '.go.sum']),
    ],
    ignore=go_ignores,
)
k8s_yaml(patch_resources(kustomize('config/default')), allow_duplicates=True)
k8s_resource(
    'raptor-controller-core',
    port_forwards=["60000:60000", "2345:2345"],
    labels=["raptor"],
    resource_deps=["redis"],
)
k8s_resource(
    'raptor-historian',
    port_forwards=["2346:2345"],
    labels=["raptor"],
    resource_deps=["redis"],
)
