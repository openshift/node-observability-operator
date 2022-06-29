## Node Observability

Node Observability project is a web of controllers and node agents/daemons
that allows you to:
* enable richer debug/profiling setting on worker nodes
* collect kubelet and crio `/pprof` data

## Deploy

### Local build:

Build the agent:
```sh
git clone https://github.com/openshift/node-observability-agent.git
cd node-observability-agent
export IMG=<registry>/<username>/node-observability-agent
make push.image.rhel8
```

Build and deploy the operator:
```sh
git clone https://github.com/openshift/node-observability-operator.git
cd node-observability-operator
export IMG=<registry>/<username>/node-observability-operator:latest
make container-build container-push deploy
```

Deploy the agents using [NodeObservability custom resource](#Prepare-to-run-profiling-queries) (CR).
Link your agent image` <registry>/<username>/node-observability-agent`
in the CR.

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

To run profiling queries on a subset of worker nodes,
you should create a `NodeObservability` custom resource (CR).

The following `NodeObservability` Operator creates a daemonset to target
all the nodes with label `app: example` by running a `node-observability-agent`
pod on each of those nodes.

```yaml
apiVersion: nodeobservability.olm.openshift.io/v1alpha1
kind: NodeObservability
metadata:
  name: cluster
spec:
  labels:
    "node-role.kubernetes.io/worker": ""
  type: crio-kubelet
```

The CRIO unix socket of the underlying node is mounted on the agent pod,
thus allowing the agent to communicate with CRIO to run the pprof request.

The `kubelet-serving-ca` certificate chain is also mounted on the agent pod,
which allows secure communication between agent and node's kubelet endpoint.

__Important__: The `NodeObservability` custom resource (CR) is unique cluster-wide. 
The operator expects the CR's name to be `cluster`, and ignores `NodeObservability` 
resources created with a different name.

`NodeObservability` resources created with a different name will be ignored
by the operator with `Ready` condition set to false in its `Status`:
```yaml
apiVersion: nodeobservability.olm.openshift.io/v1alpha1
kind: NodeObservability
metadata:
  name: anything-but-cluster
spec:
  labels:
    "node-role.kubernetes.io/worker": ""
  type: crio-kubelet
status:
  conditions:
    conditions:
    - lastTransitionTime: "2022-06-08T12:17:15Z"
      message: a single NodeObservability with name 'cluster' is authorized. Resource
        clustxxer will be discarded
      reason: Invalid
      status: "False"
      type: Ready
```

## Run profiling queries

Profiling query is a blocking operation and contains about 30 seconds
worth of profiling (kubelet + crio `/pprof`) data. You can query only one request concurrently.

You can request a Profiling query by creating a `NodeObservabilityRun` resource.
For example:

```yaml
apiVersion: nodeobservability.olm.openshift.io/v1alpha1
kind: NodeObservabilityRun
metadata:
  name: nodeobservabilityrun-sample
spec:
  nodeObservabilityRef:
    name: cluster
```

_Note: `NodeObservability` resource has to exist and referenced from the Run_

After a `NodeObservabilityRun` is created, you can track the progress of the run in
the `.Status` field. At first, `StartTimestamp` is recorded and when the run has
finished, the `FinishedTimestamp` is recorded. Any failed nodes are tracked in
`FailedAgents` list.

```yaml
$ oc get NodeObservabilityRun -o yaml --watch
apiVersion: nodeobservability.olm.openshift.io/v1alpha1
kind: NodeObservabilityRun
metadata:
  name: nodeobservabilityrun-sample
spec:
  nodeObservabilityRef:
    name: cluster
...
apiVersion: nodeobservability.olm.openshift.io/v1alpha1
kind: NodeObservabilityRun
metadata:
  name: nodeobservabilityrun-sample
spec:
  nodeObservabilityRef:
    name: cluster
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
    name: cluster
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

As of now the data is stored in the container file system under `/run`.

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

This section describes a high level "howto troubleshoot" when
encountering problems with the deployment of Node Observability.
It is by no means an exhaustive list of problems but should help
the user to navigate the common errors experienced in getting the
operator to work.

#### Node Observability Operator pod doesn't start

Images - check that `Deployment` `node-observability-operator-controller-manager`
is referencing your desired images.

#### Node Observability Agent pod doesn't start

See what the problem is `oc describe pod -l app.kubernetes.io/component=node-observability-agent`

Images - check that your `NodeObservability` is referencing your desired image.
If not, delete the resource and create new one with corrected image refs.

Certificates - Node Observability relies on
[CA Service](https://docs.openshift.com/container-platform/4.10/security/certificate_types_descriptions/service-ca-certificates.html)
to provision certificates. Check that CA Service in your cluster is enabled
and running.

Check that Service `node-observability-agent` exists in your project
and has annotation `service.beta.openshift.io/serving-cert-secret-name=node-observability-agent`.
Check if Secret `node-observability-agent` exists and has tls key and certificate.

Mount crio socket - Agent pods mount crio.sock via HostPath mount.
The pods run as privileged to achieve that. A cluster-wide policy, 
preventing privileged pods in the cluster, could exist.
