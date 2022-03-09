# NodeObservability Operator [WIP]

The `NodeObservability` Operator allows you to deploy and manage [NodeObservability Agent](https://github.com/openshift/node-observability-agent), these agents will be deployed as DaemonSets (on all or slected Nodes). \
The agent is used to trigger crio and kubelet profile data to the nodes hostPath for later retrieval (this is the MVP). 

**Note**: This Operator is in the early stages of implementation, and will be changing.

## Deploying the `NodeObservability` Operator
The following procedure describes how to deploy the `NodeObservability` Operator for BareMetal
installs.

### Installing the `NodeObservability` Operator by building and pushing the Operator image to a registry
1. Build and push the Operator image to a registry:
   ```sh
   # set the envar CONTAINER_ENGINE to the preffered container manager tool (default is podman)
   $ export IMG=<registry>/<username>/node-observability-operator:latest
   $ make container-build
   $ make container-push
   ```
2. Run the following command to deploy the `NodeObservability` Operator:
    ```
    $ make deploy
    ```

### Run the `NodeObservability` locally (for developers)
1. Execute the following `make` targets
   ```sh
   $ make install
   $ make run
   # In another terminal execute the sample CR
   $ oc apply -f /config/samples/nodeobservability_v1alpha1_nodeobservability-all.yaml
   # Alternatevely you can create a new CR and change the fields accordingly
   ```
