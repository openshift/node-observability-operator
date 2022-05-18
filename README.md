# NodeObservability Operator

The NodeObservability Operator allows you to deploy and manage [NodeObservability Agent](https://github.com/openshift/node-observability-agent) on worker nodes. The agent is deployed through DaemonSets on all or selected nodes. It also triggers the crio and kubelet profile data to the nodes hostPath for later retrieval.

**Note**: This Operator is in the early stages of implementation and keeps changing.

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

### Creating the local NodeObservability

1. To create make targets, run the following command:
   ```sh
   $ make install
   $ make run
   # In another terminal execute the sample CR
   $ oc apply -f /config/samples/nodeobservability_v1alpha1_nodeobservability-all.yaml
   # Alternatevely you can create a new CR and change the fields accordingly
   ```


### Installing the `Node Observability` Operator using a custom index image on the OperatorHub
**Note**: It is recommended to use `podman` as a container engine.

**Prerequisites**
* Openshift Container Platform cluster (CodeReady Containers for development).


**Procedure**

1. Build and push the Operator image to the registry:
    ```sh
    $ export IMG=${REGISTRY}/${REPOSITORY}/node-observability-operator:${VERSION}
    $ make container-build container-push
    ```

2. Build and push the bundle image to the registry:

    a. Add the created Operator image in the `node-observability-operator_clusterserviceversion.yaml` file:
    ```sh
    $ sed -i "s|quay.io/openshift/origin-node-observability-operator:latest|${IMG}|g" bundle/manifests/node-observability-operator.clusterserviceversion.yaml
    ```
    b. Build the image:
    ```sh
    $ export BUNDLE_IMG=${REGISTRY}/${REPOSITORY}/node-observability-operator-bundle:${VERSION}
    $ make bundle-build bundle-push
    ```

3. Build and push the index image to the registry:
   ```sh
   $ export INDEX_IMG=${REGISTRY}/${REPOSITORY}/node-observability-operator-bundle-index:${VERSION}
   $ make index-image-build index-image-push
   ```

4. (Optional) If the image is not made public, then you have to link the registry secret to the pod of the `node-observability-operator` created in the `openshift-marketplace` namespace:

    a. Create a secret with authentication details of your image registry:
    ```sh
    $ oc -n openshift-marketplace create secret generic nodeobs-olm-secret  --type=kubernetes.io/dockercfg  --from-file=.dockercfg=${XDG_RUNTIME_DIR}/containers/auth.json
    ```
    b. Link the secret to the `default` service account:
    ```sh
    $ oc -n openshift-marketplace secrets link default nodeobs-olm-secret --for=pull
    ````

5. Create the `CatalogSource` object:
   ```
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

6. Create the Operator namespace:
    ```sh
    $ oc create namespace node-observability-operator
    ```

**From the CLI**

1. Create the `OperatorGroup` object to scope the Operator to `node-observability-operator` namespace:
    ```
    cat <<EOF | oc apply -f -
    apiVersion: operators.coreos.com/v1
    kind: OperatorGroup
    metadata:
      name: node-observability-operator
      namespace: node-observability-operator
    spec:
      targetNamespaces:
      - node-observability-operator
    EOF
    ```

2. Create the `Subscription` object:
    ```
    cat <<EOF | oc apply -f -
    apiVersion: operators.coreos.com/v1alpha1
    kind: Subscription
    metadata:
      name: node-observability-operator
      namespace: node-observability-operator
    spec:
      channel: alpha
      name: node-observability-operator
      source: node-observability-operator
      sourceNamespace: openshift-marketplace
    EOF
    ```
**From the UI**

To install the Node Observability Operator from the web console, follow these steps:

1. Log in to the OpenShift Container Platform web console.

2. Navigate to **Operators → OperatorHub**.

3. Type **Node Observability Operator** into the filter box and select it.

4. Click **Install**.

5. On the Install Operator page, select A specific namespace on the cluster. Select **node-observability-operator** from the drop-down menu.


Once finished, the Node Observability Operator will be listed in the Installed Operators section of the web console.

**Verification**

* Use the following commands to verify that the Node Observability Operator has been installed.
```sh
$ oc get catalogsource -n openshift-marketplace
$ oc get operatorgroup -n node-observability-operator
$ oc get subscription -n node-observability-operator
```
