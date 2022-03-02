# NodeObservability Operator [WIP]

The `NodeObservability` Operator allows you to deploy and manage [NodeObservability Agent](https://github.com/openshift/node-observability-agent), these agents will be deployed as DaemonSets (on all or slected Nodes). The agent is used to trigger crio and kubelet profile data to the containers hostPath for later retrieval (this is the MVP). 

**Note**: This Operator is in the early stages of implementation.

## Deploying the `NodeObservability` Operator
The following procedure describes how to deploy the `NodeObservability` Operator for BareMetal
installs.

### Installing the `NodeObservability` Operator by building and pushing the Operator image to a registry
1. Build and push the Operator image to a registry:
   ```sh
   $ export IMG=<registry>/<username>/node-observability-operator:latest
   $ make container-build
   $ make container-push
   ```
2. Run the following command to deploy the `NodeObservability` Operator:
    ```
    $ make deploy
    ```

### Installing the `NodeObservability` Operator using a custom index image on OperatorHub
**Note**: By default container engine used is docker but you can specify podman by adding CONTAINER_ENGINE=podman to your image build and push commands as mentioned below.
    
1. Build and push the operator image to the registry.
   
    a. Select the container runtime you want. Either podman or docker. 
    ```sh
    $ export CONTAINER_ENGINE=podman
    ```
    b. Set your image name:
    ```sh
    $ export IMG=<registry>/<username>/node-observability-operator:latest
    ```
    c. Build and push the image:
    ```sh
    $ make container-build
    $ make container-push
    ```
   
2. Build the bundle image.
  
    a. Export your expected image name which shall have your registry name in an env var.
    ```sh
    $ export BUNDLE_IMG=<registry>/<username>/node-observability-operator-bundle:latest
    ```
    b. In the `bundle/manifests/node-observability-operator_clusterserviceversion.yaml`
        add the operator image created in Step 1 as follows:
    ```sh
        In annotations:
        Change containerImage: quay.io/openshift/origin-node-observability-operator:latest
        to containerImage: <registry>/<username, repository name>/node-observability-operator:latest
    
        In spec:
        Change image: quay.io/openshift/origin-node-observability-operator:latest
        to image: <registry>/<username, repository name>/node-observability-operator:latest
    ```
    c. Build the image
    ```sh   
    $ make bundle-build
    ```
   
3. Push the bundle image to the registry:
    ```sh
    $ make bundle-push
    ```

4. You may need to link the registry secret to the pod of `node-observability-operator` created in the `openshift-marketplace` namespace if the image is not made public ([Doc link](https://docs.openshift.com/container-platform/4.9/openshift_images/managing_images/using-image-pull-secrets.html#images-allow-pods-to-reference-images-from-secure-registries_using-image-pull-secrets)). If you are using `podman` then these are the instructions:

    a. Login to your registry:
    ```sh
    $ podman login quay.io
    ```
    b. Create a secret with registry auth details:
    ```sh
    $ oc -n openshift-marketplace create secret generic extdns-olm-secret  --type=kubernetes.io/dockercfg  --from-file=.dockercfg=${XDG_RUNTIME_DIR}/containers/auth.json
    ```
    c. Link the secret to default and builder service accounts:
    ```sh
    $ oc secrets link builder extdns-olm-secret -n openshift-marketplace
    $ oc secrets link default extdns-olm-secret --for=pull -n openshift-marketplace
    ````
    **Note**: the secret to the pod of `node-observability-operator` is part of the bundle created in step 1.


5. Create the `Catalogsource` object:

   ```bash
   $ cat <<EOF | oc apply -f -
   apiVersion: operators.coreos.com/v1alpha1
   kind: CatalogSource
   metadata:
     name: node-observability-operator
   spec:
     sourceType: grpc
     image: <registry>/<username>/node-observability-operator-bundle-index:1.0.0
   EOF
   ```

6. Create the operator namespace:
    ```bash
    oc create namespace node-observability-operator
    ```

7. Create the `Subscription` object:
    ```bash
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

8. Create the `OperatorGroup` object:
    ```bash
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

**Note**: The steps starting from the 8th can be replaced with the following actions in the web console: Navigate to  `Operators` -> `OperatorHub`, search for the `NodeObservability Operator`,  and install it in the `node-observability-operator` namespace.

### Running end-to-end tests manually

1. Deploy the operator as described above

2. Run the test suite
   ```sh
   $ make test-e2e
   ```

### Proxy support

[Configuring proxy support for NodeObservability Operator](./docs/proxy.md)
