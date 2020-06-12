# remove previous stuff
kubectl delete -f service.yaml
docker image rm gcr.io/yifeiexperiment/knative-continuous-delivery:v2
yes | gcloud container images delete gcr.io/yifeiexperiment/knative-continuous-delivery:v2

# build the Docker image and upload it to GCR
docker build -t gcr.io/yifeiexperiment/knative-continuous-delivery:v2 .
docker push gcr.io/yifeiexperiment/knative-continuous-delivery:v2

# deploy it
kubectl apply -f service.yaml
kubectl get pods
