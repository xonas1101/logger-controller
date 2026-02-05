> Right now, only Pods are being watched, slowly but surely, other resources will be watched.
> Not only logging, metrics will also be exposed for all resources.

# Logger Controller

A Kubernetes **observer-style controller** that watches **Pods** and **Deployments** and logs their state based on a declarative Custom Resource (`Logger`).

Built using **Kubebuilder / controller-runtime**, this project focuses on reconciliation, watches, and logging patterns rather than resource mutation.

---

## What this controller does

- Defines a `Logger` Custom Resource
- Watches **Pod** and **Deployment** events (create / update / delete)
- On every Pod/Deplyment event:
  - Reconciles matching `Logger` resources
  - Logs the current state of Pods/Deployments
- Supports:
  - Namespace-scoped or cluster-scoped logging
  - Exclusion of Kubernetes system namespaces
- Does **not** modify Pods/Deployments or any cluster resources

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
  trigger: {}
