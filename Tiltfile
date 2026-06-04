version_settings(constraint=">=0.22.2")
secret_settings(disable_scrub=True)
load('ext://dotenv', 'dotenv')

# Load tilt env file if it exists
dotenv_path = ".tilt.env"
if os.path.exists(dotenv_path):
  dotenv(fn=dotenv_path)

# Configure trigger mode
true = ("true", "1", "yes", "t", "y")
trigger_mode = TRIGGER_MODE_MANUAL
if os.environ.get('TRIGGER_MODE_AUTO', '').lower() in true:
  trigger_mode = TRIGGER_MODE_AUTO

# Docker images
custom_build(
  ref="sfa-enduro-worker:dev",
  command=["hack/build_docker.sh", "sfa-enduro-worker"],
  deps=["."],
)
custom_build(
  ref="sfa-dips:dev",
  command=["hack/build_docker.sh", "sfa-dips"],
  deps=["."],
)
docker_build(ref="apis-mock:dev", context="hack/apis-mock")

# Kubernetes manifests
kube_objects = decode_yaml_stream(kustomize("hack/kube"))
for obj in kube_objects:
  if (
    obj.get("kind") == "Deployment" and
    obj.get("metadata", {}).get("name") == "apis-mock"
  ):
    env = obj["spec"]["template"]["spec"]["containers"][0]["env"]
    env[0]["value"] = os.environ.get("MOCK_ANALYSIS_RESULT") or env[0]["value"]
    env[1]["value"] = os.environ.get("MOCK_IMPORT_RESULT") or env[1]["value"]

k8s_yaml(encode_yaml_stream(kube_objects))

# Tilt resources
k8s_resource(
  "sfa-enduro-worker",
  labels=["SFA"],
  trigger_mode=trigger_mode,
)
k8s_resource(
  "sfa-dips",
  port_forwards="8082:8080",
  labels=["SFA"],
  trigger_mode=trigger_mode,
)
k8s_resource(
  "apis-mock",
  port_forwards="8081:8080",
  labels=["SFA"],
  trigger_mode=trigger_mode,
)
k8s_resource("mysql-create-prep-database", labels=["SFA"])
k8s_resource(
  "mysql-recreate-prep-database",
  labels=["SFA"],
  auto_init=False,
  trigger_mode=TRIGGER_MODE_MANUAL,
)
