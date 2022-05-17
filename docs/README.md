## Node Observability

## Deploy

### Local deployment:

```
git clone https://github.com/openshift/node-observability-agent.git
git clone https://github.com/openshift/node-observability-operator.git

IMG=<registry>/<username>/node-observability-operator:latest make container-build container-push deploy
```

### Deployment from a public release

1. Create the Operator namespace:
```sh
oc new-project node-observability-operator
```

2. Login to the OpenShift Container Platform web console
3. Navigate to Operators → OperatorHub.
Type Node Observability Operator into the filter box and select it.
4. Click Install

### Deployment from a custom catalog

1. Prepare an index image
```sh
export BUNDLE_IMG=<registry>/<username>/node-observability-operator-bundle:<version>
export INDEX_IMG=<registry>/<username>/node-observability-operator-bundle-index:<version>
make index-image-build index-image-push
```
2. Ensure image registry is accessible,
   1. by declaring the necessary credentials as secrets,
   ```sh
   oc -n openshift-marketplace create secret generic nodeobs-olm-secret  --type=kubernetes.io/dockercfg  --from-file=.dockercfg=${XDG_RUNTIME_DIR}/containers/auth.json
   oc -n openshift-marketplace secrets link default nodeobs-olm-secret --for=pull
   ```
   2. Adding an `ImageContentSourcePolicy` if needed
   ```sh
   cat <<EOF  | oc create -f -
   apiVersion: operator.openshift.io/v1alpha1
   kind: ImageContentSourcePolicy
   metadata:
     name: brew-registry
   spec:
     repositoryDigestMirrors:
     - mirrors:
       - brew.registry.redhat.io
       source: registry.redhat.io
     - mirrors:
       - brew.registry.redhat.io
       source: registry.stage.redhat.io
     - mirrors:
       - brew.registry.redhat.io
       source: registry-proxy.engineering.redhat.com
   EOF
   ```
3. Create a CatalogSource object:
```sh
cat <<EOF | oc apply -f - 
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: node-observability-operator
  namespace: openshift-marketplace
spec:
  sourceType: grpc
  image: ${INDEX_IMG}
EOF
```
4. Follow same steps as deployment from public release (see above)

## Prepare to run profiling queries

To run profiling queries on a subset of worker nodes, you should create a `NodeObservability` custom resource (CR).

The following `NodeObservability` will create a daemonset targetting all nodes with label `app: example`, running a `node-observability-agent` pod on each of those node using the image given in `.spec.image`.

```yaml
apiVersion: nodeobservability.olm.openshift.io/v1alpha1
kind: NodeObservability
metadata:
  name: nodeobservability-sample
  namespace: node-observability-operator
spec:
  labels:
    "node-role.kubernetes.io/worker": ""
  image: "brew.registry.redhat.io/rh-osbs/node-observability-agent:0.1.0-3"
```

The CRIO unix socket of the underlying node is mounted on the agent pod, thus allowing the agent to communicate with CRIO to run the pprof request.

The `kubelet-serving-ca` certificate chain is also mounted on the agent pod, which allows secure communication between agent and node's kubelet endpoint.

## Run profiling queries

Profiling query is a blocking operation and contains about 30 seconds
worth of profiling (kubelet + crio `/pprof`) data. As such, only one
query can be requested concurrently.

Profiling queries can be requested with creating a `NodeObservabilityRun`
resource. For example:

```yaml
apiVersion: nodeobservability.olm.openshift.io/v1alpha1
kind: NodeObservabilityRun
metadata:
  name: nodeobservabilityrun-sample
spec:
  nodeObservabilityRef:
    name: nodeobservability-sample
```

_Note: `NodeObservability` resource has to exist and referenced from the Run_

Once a `NodeObservabilityRun` is created, progress of the run is tracked in
the `.Status` field. First, `StartTimestamp` is recorded and when the run has
finished, the `FinishedTimestamp` is recorded. Any failed nodes are tracked in
`FailedAgents` list.

```
$ oc get NodeObservabilityRun -o yaml --watch
apiVersion: nodeobservability.olm.openshift.io/v1alpha1
kind: NodeObservabilityRun
metadata:
  name: nodeobservabilityrun-sample
spec:
  nodeObservabilityRef:
    name: nodeobservability-sample
...
apiVersion: nodeobservability.olm.openshift.io/v1alpha1
kind: NodeObservabilityRun
metadata:
  name: nodeobservabilityrun-sample
spec:
  nodeObservabilityRef:
    name: nodeobservability-sample
status:
  startTimestamp: 2022-05-12T15:25:54.192343392+02:00
  agents:
  - name: ip-172-31-83-20.ec2.internal
    ip: 172.31.83.20
    port: 8443
  - name: ip-172-31-83-22.ec2.internal
    ip: 172.31.83.22
    port: 8443
...
apiVersion: nodeobservability.olm.openshift.io/v1alpha1
kind: NodeObservabilityRun
metadata:
  name: nodeobservabilityrun-sample
spec:
  nodeObservabilityRef:
    name: nodeobservability-sample
status:
  startTimestamp: 2022-05-12T15:25:54.192343392+02:00
  finishedTimestamp 2022-05-12T15:26:25.192343392+02:00
  agents:
  - name: ip-172-31-83-20.ec2.internal
    ip: 172.31.83.20
    port: 8443
  - name: ip-172-31-83-22.ec2.internal
    ip: 172.31.83.22
    port: 8443
  output: /run/node-observability/50778b44-d1f8-11ec-9d64-0242ac120002
```

Data retrieval is currently in development.

Right now the data is stored in the container file system under `/run`.

With a nodeobservabilityrun called `nodeobservabilityrun-sample`:

```sh
for i in `oc get nodeobservabilityrun.nodeobservability.olm.openshift.io/nodeobservabilityrun-sample -o yaml | yq .status.agents[].name | cut -d\" -f2`
  do
  echo $i
  list=`oc exec $i -c node-observability-agent -- bash -c "ls /run/*.pprof"`
  for j in $list
    do
    k=`echo $j|cut -d\/ -f3`
    mkdir -p /tmp/$i
    echo copying $k to /tmp/$i/$k
    kubectl exec $i -c node-observability-agent -- cat $j > /tmp/$i/$k
  done
done
```

## Troubleshooting
