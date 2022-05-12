## Node Observability

### Deploy

### Run profiling queries

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
