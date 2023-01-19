load('ext://helm_resource', 'helm_resource', 'helm_repo')
load('ext://cert_manager', 'deploy_cert_manager')
load('ext://restart_process', 'docker_build_with_restart')

## Configs
config.define_bool('with-cert-manager', usage='Deploy cert-manager')
config.define_bool('with-ngnix-ingress', usage='Deploy nginx-ingress')
config.define_bool('init-samples', usage='Deploy sample resources when initializing')
cfg = config.parse()

print("👨‍💻👩‍💻Development environment")
print(' Running with config: %s' % cfg)

if cfg.get('with-cert-manager', False):
    print("→ Deploying cert-manager")
    deploy_cert_manager()

if cfg.get('with-ngnix-ingress', False):
    print("→ Deploying nginx-ingress")
    kind_yaml = decode_yaml_stream(local(
        'curl https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml 2>/dev/null',
        quiet=True, echo_off=True))
    for o in kind_yaml:
        if o['kind'] == 'Deployment':
            o['spec']['template']['spec']['nodeSelector'] = {'kubernetes.io/os': 'linux'}
    kind_yaml = encode_yaml_stream(kind_yaml)
    k8s_yaml(kind_yaml)
    k8s_resource(new_name='ingress-nginx-ns', objects=['ingress-nginx:namespace'],
                 labels=['cluster-services', 'ingress'])
    k8s_resource(
        'ingress-nginx-controller',
        port_forwards=['8080:80', '8443:443'],
        labels=['cluster-services', 'ingress'],
        resource_deps=['ingress-nginx-ns'],
    )
    k8s_resource('ingress-nginx-admission-create', resource_deps=['ingress-nginx-ns'],
                 labels=['cluster-services', 'ingress'])
    k8s_resource('ingress-nginx-admission-patch', resource_deps=['ingress-nginx-ns'],
                 labels=['cluster-services', 'ingress'])

print("→ Deploying Redis")
helm_repo('ot-helm', 'https://ot-container-kit.github.io/helm-charts/', labels=['cluster-services'])
helm_resource(
    'redis-operator',
    'ot-helm/redis-operator',
    namespace='ot-operators',
    flags=['--create-namespace'],
    labels=['cluster-services', 'redis'],
    resource_deps=['ot-helm'],
)
k8s_kind("Redis")
k8s_yaml('./hack/redis-standalone.yaml')
k8s_resource(
    'redis-standalone',
    port_forwards=['6379:6379'],
    labels=['redis', 'cluster-services'],
    resource_deps=['redis-operator'],
)

# Install prometheus-operator
print("→ Deploying Prometheus")
helm_repo('prometheus-community', 'https://prometheus-community.github.io/helm-charts', labels=['cluster-services'])
helm_resource(
    'prometheus',
    'prometheus-community/kube-prometheus-stack',
    namespace='monitoring',
    labels=['cluster-services'],
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

### Raptor
print("🏗️ Building Raptor images")

def patch_resources(yaml):
    resources = decode_yaml_stream(yaml)
    for resource in resources:
        if resource['kind'] == 'Deployment':
            if resource['spec']['replicas'] > 1:
                resource['spec']['replicas'] = 1

            resource['spec']['template']['spec']['securityContext'] = {}
            for container in resource['spec']['template']['spec']['containers']:
                container['securityContext'] = {}

    return encode_yaml_stream(resources)


#### CRD
k8s_yaml(kustomize('config/crd'), allow_duplicates=True)
k8s_kind("Model")
k8s_kind("Feature")
k8s_kind("DataSource")

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
    entrypoint=['python', '/runtime/runtime.py'],
    trigger=['runtime/'],
    ignore=py_ignores,
)

#### Controllers
base_dockerfile = '''
FROM golang:1.19 AS build
RUN go install github.com/go-delve/delve/cmd/dlv@latest

WORKDIR /workspace
COPY go.mod /workspace
COPY go.sum /workspace
RUN go mod download

COPY . /workspace

'''

compile_cmd = '''CGO_ENABLED=0 go build -gcflags 'all=-N -l' -o /opt/{app} cmd/{app}/*.go'''
dlv_cmd = [
    '/go/bin/dlv',
    '--listen=0.0.0.0:2345',
    '--api-version=2',
    '--headless=true',
    '--only-same-user=false',
    '--accept-multiclient',
    '--check-go-version=false',
    '--log',
    '--check-go-version=false',
    'exec',
    '--continue',
    '--',
]

##### Core
docker_build_with_restart(
    'raptor-core',
    '.',
    dockerfile_contents=base_dockerfile + 'RUN ' + compile_cmd.format(app='core'),
    entrypoint=dlv_cmd + [
        '/opt/core',
        '--health-probe-bind-address=:8081',
        '--metrics-bind-address=127.0.0.1:8080',
        '--leader-elect',
        '-r=redis-standalone.default:6379',
        '--zap-devel=true',
    ],
    live_update=[
        # Copy the binary so it gets restarted.
        sync(
            '.', '/workspace'
        ),
        run(compile_cmd.format(app='core')),
        run('go mod download', trigger=['.go.mod', '.go.sum']),
    ],
    ignore=go_ignores,
)

##### Historian
docker_build_with_restart(
    'raptor-historian',
    '.',
    dockerfile_contents=base_dockerfile + 'RUN ' + compile_cmd.format(app='historian'),
    entrypoint=dlv_cmd + [
        '/opt/historian',
        '-r=redis-standalone.default:6379',
        '--zap-devel=true',
    ],
    live_update=[
        # Copy the binary so it gets restarted.
        sync(
            '.', '/workspace'
        ),
        run(compile_cmd.format(app='historian'), trigger=[
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
    port_forwards=['60000:60000', '2345:2345'],
    labels=['raptor'],
    resource_deps=['redis-standalone'],
)
k8s_resource(
    'raptor-historian',
    port_forwards=['2346:2345'],
    labels=['raptor'],
    resource_deps=['redis-standalone'],
)

for sample in decode_yaml_stream(kustomize('config/samples')):
    name = sample['metadata']['name']
    k8s_yaml(encode_yaml(sample))
    k8s_resource(
        name,
        labels=['raptor-samples'],
        resource_deps=['raptor-controller-core'],
        auto_init=cfg.get('init-samples', False),
    )
