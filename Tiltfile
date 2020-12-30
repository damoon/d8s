disable_snapshots()
allow_k8s_contexts(['test', 'ci'])

load('ext://min_tilt_version', 'min_tilt_version')
min_tilt_version('0.15.0') # includes fix for auto_init+False with tilt ci

include('./service-dependencies/Tiltfile')
include('./tests/Tiltfile')

k8s_yaml('deployment/config.yaml')
k8s_yaml('deployment/kubernetes.yaml')

target='prod'
live_update=[]
if os.environ.get('PROD', '') ==  '':
  target='build-env'
  live_update=[
    sync('pkg',    '/app/pkg'),
    sync('cmd',    '/app/cmd'),
    sync('go.mod', '/app/go.mod'),
    sync('go.sum', '/app/go.sum'),
    run('go install ./cmd/wedding'),
  ]

docker_build(
  'davedamoon/wedding:latest',
  '.',
  dockerfile='deployment/Dockerfile',
  target=target,
  build_args={"SOURCE_BRANCH":"development", "SOURCE_COMMIT":"development"},
  only=[ 'go.mod'
       , 'go.sum'
       , 'pkg'
       , 'cmd'
       , 'deployment'
  ],
  ignore=[ '.git'
         , '*/*_test.go'
         , 'deployment/kubernetes.yaml'
  ],
  live_update=live_update,
)

k8s_resource(
  'wedding',
  port_forwards=['12376:2376'],
  resource_deps=['minio-buckets', 'registry'],
)
