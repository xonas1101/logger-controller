[![ACMM](https://img.shields.io/endpoint?url=https%3A%2F%2Fconsole.kubestellar.io%2Fapi%2Facmm%2Fbadge%3Frepo%3Dxonas1101%252Flogger-controller)](https://console.kubestellar.io/acmm?repo=xonas1101%2Flogger-controller)

> Right now, only Pods, Deployments and ReplicaSets are being watched, slowly but surely, other resources will be watched.
> Not only logging, metrics will also be exposed for all resources.
> New functionalities are otw.
> Almost time to end this project. Gonna work on my main kubernetes controller project.

# Logger Controller

A Kubernetes **observer-style controller** that watches **Pods**, **Deployments** and **ReplicaSets** and logs their state based on a declarative Custom Resource (`Logger`).

Built using **Kubebuilder / controller-runtime**, this project focuses on reconciliation, watches, and logging patterns rather than resource mutation.

---

## What this controller does

- Defines a `Logger` Custom Resource
- Watches **Pod**, **Deployment** and **ReplicaSet** events (create / update / delete)
- On every Pod/Deployment/ReplicaSet event:
  - Reconciles matching `Logger` resources
  - Logs the current state of Pods/Deployments
- Supports:
  - Namespace-scoped or cluster-scoped logging
  - Exclusion of Kubernetes system namespaces
- Does **not** modify Pods/Deployments/Replicasets or any cluster resources

This is an **observer controller**, not a CRUD controller.

---

## Why not kubectl get all?

This controller is not an alternative to ```kubectl get all```.

I started this as a learning project to understand Kubernetes controllers better
and to move closer to the kind of work platform engineers actually do. The point
for me is the experience of building and running a controller, not competing
with existing CLI commands.


---

## Design principles

- Custom Resources are **configuration**, not workloads
- Reconcile loops are **read-only**
- Pod events, pre-defined Intervals drive reconciliation
- Logs are structured and intentional

---

## How to use

This controller works on a kubernetes cluster (you guessed it), for the same we need a cluster provisioner, like ```kind```, ```k3d``` or others.

Make sure you have:

- **Go** (≥ 1.20)
- **Docker**
- **kubectl**
- **kind**
- **make**

Once you have a running version of any one of the above, go ahead and provision a cluster.

Make sure that the context of kubectl is set to your target cluster, if you have multiple clusters. To check, run 
```kubectl config current-context```, and inspect if the output is the same as your target cluster.

Next, we want to install our CRD into our cluster. For that, an example CRD file has been provided, ```example/crd.yaml```. Make the changes you want and run the 
```make install``` command from repo root.

Go ahead and make some resources (Pods/Deployments/Replicasets) on your cluster, which you want observed.

Then, make CR for you CRD, using the command 
```kubectl create -f example/crd.yaml``` from repo root.

We are all done. All thats left is to run the controller itself. For that, go to repo root and run 
```go run cmd/main.go``` and voila your controller has started.

Note:
For logs like that in ```kubectl get all```, use
```go run cmd/main.go --zap-log-level=info``` from repo root.
For in depth, detailed logs, use
```go run cmd/main.go --zap-log-level=debug``` from repo root.

---

## Logger Custom Resource

### Example

```yaml
apiVersion: logger.logger.com/v1
kind: Logger
metadata:
  name: logger-sample
  namespace: default
spec:
  scope:
    type: namespace        # "namespace" or "cluster"
    namespace: default     # required if type=namespace
  resources:
    - pods
    - deployments
    - replicasets
  trigger: 30s


