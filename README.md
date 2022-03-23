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

### Creating the local NodeObservability

1. To create make targets, run the following command:
   ```sh
   $ make install
   $ make run
   # In another terminal execute the sample CR
   $ oc apply -f /config/samples/nodeobservability_v1alpha1_nodeobservability-all.yaml
   # Alternatevely you can create a new CR and change the fields accordingly
   ```


### Installing the `Node Observability` Operator using a custom index image on OperatorHub
**Note**: The below procedure works best with `podman` as container engine
    
1. Build and push the operator image to the registry:
    ```sh
    export IMG=${REGISTRY}/${REPOSITORY}/node-observability-operator:${VERSION}
    make container-build container-push
    ```

2. Build and push the bundle image to the registry:
  
    a. In the `bundle/manifests/node-observability-operator_clusterserviceversion.yaml`
        add the operator image created in Step 1 as follows:
    ```sh
    sed -i "s|quay.io/openshift/origin-node-observability-operator:latest|${IMG}|g" bundle/manifests/node-observability-operator_clusterserviceversion.yaml
    ```
    b. Build the image
    ```sh
    export BUNDLE_IMG=${REGISTRY}/${REPOSITORY}/node-observability-operator-bundle:${VERSION}
    make bundle-build bundle-push
    ```

3. Build and push the index image to the registry:
   ```sh
   export INDEX_IMG=${REGISTRY}/${REPOSITORY}/node-observability-operator-bundle-index:${VERSION}
   make index-image-build index-image-push
   ```

4. You may need to link the registry secret to the pod of `node-observability-operator` created in the `openshift-marketplace` namespace if the image is not made public ([Doc link](https://docs.openshift.com/container-platform/4.10/openshift_images/managing_images/using-image-pull-secrets.html#images-allow-pods-to-reference-images-from-secure-registries_using-image-pull-secrets)). If you are using `podman` then these are the instructions:

    a. Create a secret with authentication details of your image registry:
    ```sh
    oc -n openshift-marketplace create secret generic nodeobs-olm-secret  --type=kubernetes.io/dockercfg  --from-file=.dockercfg=${XDG_RUNTIME_DIR}/containers/auth.json
    ```
    b. Link the secret to `default` service account:
    ```sh
    oc -n openshift-marketplace secrets link default nodeobs-olm-secret --for=pull
    ````

6. Create the `CatalogSource` object:
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

7. Create the operator namespace:
    ```sh
    oc create namespace node-observability-operator
    ```

8. Create the `OperatorGroup` object to scope the operator to `node-observability-operator` namespace:
    ```sh
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

9. Create the `Subscription` object:
    ```sh
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

**Note**: The steps starting from the 7th can be replaced with the following actions in the web console: Navigate to  `Operators` -> `OperatorHub`, search for the `Node Observability Operator`,  and install it in the `node-observability-operator` namespace.
