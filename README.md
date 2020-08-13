**This is not an officially supported Google product.**

# Knative Continuous Delivery User Guide

## What does KCD do?

Knative Continuous Delivery automates progressive rollout of cloud services in Knative. You can choose one of the predefined rollout policies or define a new policy on your own, and the KCD controller will make sure that all services you deploy using that policy will follow the rollout traffic behavior. This greatly reduces rollout risk when introducing new versions of your product, and can help improve continuous delivery during development.

## Use Cases & Expected Behavior

### Deploying your product using Knative Service

KCD supports progressive deployment if you deploy your services using Knative Service. You should specify a rollout policy for a Service using annotations like so:

```yaml
apiVersion: serving.knative.dev/v1alpha1
kind: Service
metadata:
  ...
  annotations:
    delivery.knative.dev/policy: <your_policy>
...
```

Note that you must first create a valid policy with the correct name, otherwise the progressive rollout will not initiate because it will not be able to find the correct policy resource with the given name. The newest Revision created for this Service will follow the policy you specified, slowly increasing its traffic percentage over time. All the older Revisions will have their routing percentages adjusted as appropriate.

By default, KCD searches for the Policy in the same namespace as the Service. However, you can optionally instruct KCD to perform a cross-namespace policy query by prefixing your policy name with another namespace:

```
delivery.knative.dev/policy: other-namespace/your-policy-name
```

For this reason, the forward slash character `/` is specially reserved. You should avoid naming your policies or namespaces with it.

### Deploying your product using Knative Route and Configuration

If you are not using Knative Service to deploy your service, KCD also supports Route and Configurations (if you use Knative Service, then Route and Configuration will be automatically created by Knative). You should annotate your Configuration resource with the correct namespace and policy name (as shown above). Note that cross-namespace policy queries are also supported in this use case.

### Deployment multiple Revisions simultaneously

KCD can handle more than 2 active Revisions for a service at the same time. It will always satisfy the traffic demands of the latest Revision first, followed by the second latest Revision, third latest Revision, and so on. When 100% of traffic has been assigned to a set of Revisions, any older Revisions that are not included in this pool will be eliminated as a traffic target. An in-depth example demonstrating the routing behavior when multiple Revisions are simultaneously "in flight" can be found [here](multiple_revision_example.md)

## Rollout Policies

### Example Policies

KCD comes with some predefined, example policies for you to play with. One example is the following yaml manifest of a policy:

```yaml
apiVersion: delivery.knative.dev/v1alpha1
kind: Policy
metadata:
  namespace: default
  name: advanced-policy
spec:
  mode: time
  defaultThreshold: 60
  stages:
  - percent: 25
    threshold: 30
  - percent: 50
    threshold: 30
  - percent: 75
```

The `mode` field specifies how the threshold values should be interpreted. A time-based policy implies that the threshold values should be interpreted with units of seconds. A threshold value is used to determine when to move the rollout into the next stage. The first rollout stage in the example above has a rollout threshold of 30 seconds, meaning that 30 seconds after the rollout enters this stage, it will progress to the next stage.

The `defaultThreshold` field specifies what the threshold value for a stage should be if that stage does not specify an optional threshold value on its own. For example, the third stage in the example above (with 75%) does not have an optional threshold value, so its threshold is 60 seconds.

Each `percent` in the list of stages specify the rollout percentage for that stage, i.e. the percentage of traffic that the newest Revision should be receive at this stage.

### Customizing Policies

Although you can choose to use one of the example policies directly, you can also write and apply rollout policies on your own. To do so, you need a yaml file that describes your policy in a format similar to the one shown above.

When applying policies, the following criteria must be met (otherwise your policy will be rejected and will have no effect):

1. The value of the `mode` field must be valid (currently only "time" is accepted).
2. The value of `defaultThreshold` must be a positive integer, and `defaultThreshold` cannot be omitted.
3. There must be at least one rollout stage.
4. The values of `percent` fields must be in non-decreasing order.
5. The values of `percent` fields must be in the range [1, 99]. You do not need to specify 0 as the first stage, and you MUST not specify 100 as the final stage.
6. Any optional `threshold` values, if specified, must be positive integers.

## Example Workflow

### Installing KCD

Before installing and using KCD, you need to make sure that you have `go`, `kubectl`, and `ko` installed on your machine. Your cluster must also have Knative Serving.

To install KCD, run the following command in your terminal:

```sh
ko apply -f config/
```

This will create all the prerequisite custom resource definitions and deploy the KCD controller and webhook to your cluster. To verify that the controller and webhook have been correctly deployed, you can use the following command: 

```sh
kubectl get pod -n knative-serving
```

In the printout, if you can find two lines similar to the ones below, then your KCD controller and webhook are correctly deployed:

```
NAME                                                            READY   STATUS    RESTARTS   AGE
continuous-delivery-webhook-677cf69bcd-k4lls                    1/1     Running   0          89s
knative-continuous-delivery-dg8g6-deployment-6d97dd9869-r6hp9   1/2     Running   0          88s
```

### Applying your Policy or use a built-in Policy

A few built-in policies have been provided in `example-policies/`. You can apply them by doing

```sh
ko apply -f example-policies
```

Note that to actually use a built-in policy in a rollout, the relevant Service or Configuration must have the appropriate annotations to specify the name of the policy that it is using.

### Deploying your service

Suppose you have a Knative Service `default/my-app` in a file called `my-app.yaml` like the following:

```yaml
apiVersion: serving.knative.dev/v1alpha1
kind: Service
metadata:
  name: my-app
  annotations:
    delivery.knative.dev/policy: basic-policy
spec:
  runLatest:
    configuration:
      revisionTemplate:
        spec:
          container:
            image: <some image>
```

then you can simply run the following command to deploy it:

```sh
kubectl apply -f my-app.yaml
```

If this is the first Revision of `my-app`, then the behavior is the same as if KCD is not present. However, if this is not the first Revision of `my-app`, and if `my-app.yaml` correctly identifies a rollout policy in the annotations ("basic-policy" in this case, which refers to one of the two example policies), then KCD will automatically compute the correct routing state for `my-app` and initiate a progressive rollout. You can watch the traffic percentages in real time via the following command:

```sh
kubectl get -w route my-app -o yaml
```

## Common Failures & Diagnostics

### Incorrect traffic behavior

If you see that the rollout percentage for a Revision you just created stays at 0 or jumps to 100 directly, there can be a few potential culprits:

1. You have not applied any rollout policy.
2. You have not annotated your Service/Configuration with the correct policy name.
3. You are not using Knative Service, and your Route and Configuration have namespace and/or name mismatches.
4. The policy you specified does not live in the same namespace as your Service/Configuration, and you did not specify any namespace for the cross-namespace policy query.

### Debugging tips

You can view the logs of the KCD controller and/or webhook by using the following command:

```sh
kubectl logs -n knative-serving <pod_name_for_controller/webhook>
```

## Under-the-Hood Technicalities

### Continuous Delivery Webhook

The continuous delivery webhook serves two purposes:

1. To make sure that the KService reconciler does not overwrite changes to the Route object, the webhook implements a method that intercepts and overwrites the Route with the desired routing spec before the Route reaches the K8s API server.
2. To make sure that customized policies strictly adhere to all the required criteria, the webhook implements a method that valids any Policy object it receives, and rejects any errors.

### The PolicyState resource

The PolicyState custom resource is owned by the KCD controller. Its purpose is for the KCD controller to communicate the desired routing state to the continuous delivery webhook. Because the KCD controller and the webhook are not run in the same binary, PolicyStates are used to relay information. It is purely an internal mechanism that helps the webhook write the correct Route spec; any tampering with PolicyStates might cause undefined behavior.
