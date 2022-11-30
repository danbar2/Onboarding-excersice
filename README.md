
# helloworld-operator
Manage Helloworld CR's, make sure each CR has the required count of pods ready for internal GET requests on port 8080 and route /hello

## Getting Started
You’ll need a Kubernetes cluster to run against.

### Running on the cluster
1. Build and push your image to the location specified by `IMG`:
	
make docker-build docker-push IMG="gcr.io/run-ai-lab/helloworld"
	
2. Deploy the controller to the cluster with the image specified by `IMG`:

make deploy IMG="gcr.io/run-ai-lab/helloworld"

3. Deploy a Helloworld CR
kubectl apply -f config/samples/cache_v1alpha1_helloworld.yaml

### Uninstall CRDs
To delete the CRDs from the cluster:

make uninstall

### Undeploy controller
UnDeploy the controller to the cluster:

make undeploy  