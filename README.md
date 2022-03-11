# NodeObservability Operator

The NodeObservability Operator allows you to deploy and manage [NodeObservability Agent](https://github.com/openshift/node-observability-agent) on worker nodes. The agent is deployed through DaemonSets on all or selected nodes. It also triggers the crio and kubelet profile data to the nodes hostPath for later retrieval.

**Note**: This Operator is in the early stages of implementation, and will be changing.

## Deploying the `NodeObservability` Operator

You can deploy the NodeObservability Operator for a BareMetal installation by using the following procedure:

### Installing the `NodeObservability` Operator

You can install the NodeObservability Operator by building and pushing the Operator image into a registry.

1. To build and push the Operator image into a registry, run the following commands:
   ```sh
   # set the envar CONTAINER_ENGINE to the preffered container manager tool (default is podman)
   $ export IMG=<registry>/<username>/node-observability-operator:latest
   $ make container-build
   $ make container-push
   ```
2. To deploy the NodeObservability Operator, run the following command:
    ```
    $ make deploy
    ```

### Creating the local NodeObservabilitya

1. To create make targets, run the following command:
   ```sh
   $ make install
   $ make run
   # In another terminal execute the sample CR
   $ oc apply -f /config/samples/nodeobservability_v1alpha1_nodeobservability-all.yaml
   # Alternatevely you can create a new CR and change the fields accordingly
   ```
