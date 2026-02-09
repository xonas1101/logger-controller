/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	loggerv1 "github.com/xonas1101/logger-controller/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// LoggerReconciler reconciles a Logger object
type LoggerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=logger.logger.com,resources=loggers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=logger.logger.com,resources=loggers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=logger.logger.com,resources=loggers/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=replicasets,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Logger object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.1/pkg/reconcile

func podLine(p corev1.Pod) string {
	return "pod/" + p.Name + " " +
		p.Namespace + " " +
		string(p.Status.Phase) + " " +
		p.Spec.NodeName
}

func deployLine(d appsv1.Deployment) string {
	desired := int32(1)
	if d.Spec.Replicas != nil {
		desired = *d.Spec.Replicas
	}

	return "deploy/" + d.Name + " " +
		d.Namespace + " " +
		fmt.Sprintf("%d/%d", d.Status.AvailableReplicas, desired)
}

func rsLine(rs appsv1.ReplicaSet) string {
	return "rs/" + rs.Name + " " +
		rs.Namespace + " " +
		fmt.Sprintf("%d", rs.Status.Replicas)
}

func (r *LoggerReconciler) loggerRequestsForPod(
	ctx context.Context,
	obj client.Object,
) []ctrl.Request {

	// We don’t care which pod changed — we re-log everything
	var loggerList loggerv1.LoggerList
	if err := r.List(ctx, &loggerList); err != nil {
		return nil
	}

	reqs := make([]ctrl.Request, 0, len(loggerList.Items))
	for _, logger := range loggerList.Items {
		reqs = append(reqs, ctrl.Request{
			NamespacedName: client.ObjectKey{
				Name:      logger.Name,
				Namespace: logger.Namespace,
			},
		})
	}

	return reqs
}

func printKubectl(line string) {
	fmt.Println(line)
}

func (r *LoggerReconciler) loggerRequestsForDeployment(
	ctx context.Context,
	obj client.Object,
) []ctrl.Request {

	var loggerList loggerv1.LoggerList
	if err := r.List(ctx, &loggerList); err != nil {
		return nil
	}

	reqs := make([]ctrl.Request, 0, len(loggerList.Items))
	for _, logger := range loggerList.Items {
		reqs = append(reqs, ctrl.Request{
			NamespacedName: client.ObjectKey{
				Name:      logger.Name,
				Namespace: logger.Namespace,
			},
		})
	}
	return reqs
}

func isSystemNamespace(ns string) bool {
	switch ns {
	case "kube-system", "kube-public", "kube-node-lease":
		return true
	}
	return false
}

func listOptionsFromScope(logger *loggerv1.Logger) []client.ListOption {
	opts := []client.ListOption{}

	switch strings.ToLower(logger.Spec.Scope.Type) {
	case "namespace":
		if logger.Spec.Scope.Namespace != "" {
			opts = append(opts, client.InNamespace(logger.Spec.Scope.Namespace))
		}
	case "cluster":
		// no-op → cluster-wide
	default:
		// unknown scope → fallback to cluster
	}

	return opts
}

func shouldLogPods(logger *loggerv1.Logger) bool {
	return slices.Contains(logger.Spec.Resources, "pods")
}

func shouldLogDeployments(logger *loggerv1.Logger) bool {
	return slices.Contains(logger.Spec.Resources, "deployments")
}

func shouldLogReplicaSets(logger *loggerv1.Logger) bool {
	return slices.Contains(logger.Spec.Resources, "replicasets")
}

func (r *LoggerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var observed int32 = 0
	l := logf.FromContext(ctx)

	l.Info("logger reconcile")

	var logger loggerv1.Logger
	if err := r.Get(ctx, req.NamespacedName, &logger); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	requeueAfter := 30 * time.Second
	if logger.Spec.Trigger != "" {
		d, err := time.ParseDuration(logger.Spec.Trigger)
		if err != nil {
			l.Error(err, "invalid spec.trigger")
			return ctrl.Result{}, err
		}
		requeueAfter = d
	}

	opts := listOptionsFromScope(&logger)

	// ---------------- PODS ----------------
	if shouldLogPods(&logger) {
		var podList corev1.PodList
		if err := r.List(ctx, &podList, opts...); err != nil {
			return ctrl.Result{}, err
		}

		l.V(1).Info("pods observed", "count", len(podList.Items))
		printKubectl("PODS")

		for _, pod := range podList.Items {
			if isSystemNamespace(pod.Namespace) {
				continue
			}
			observed++

			// ---------- kubectl-style (dev) ----------
			printKubectl(podLine(pod))

			// ---------- ORIGINAL deep logs ----------
			l.V(1).Info(
				"pod details",
				"name", pod.Name,
				"namespace", pod.Namespace,
				"node", pod.Spec.NodeName,
				"phase", pod.Status.Phase,
				"hostIP", pod.Status.HostIP,
				"podIP", pod.Status.PodIP,
				"startTime", pod.Status.StartTime,
				"conditions", pod.Status.Conditions,
			)
		}
	}

	// ---------------- DEPLOYMENTS ----------------
	if shouldLogDeployments(&logger) {
		var deplist appsv1.DeploymentList
		if err := r.List(ctx, &deplist, opts...); err != nil {
			return ctrl.Result{}, err
		}

		l.V(1).Info("deployments observed", "count", len(deplist.Items))
		printKubectl("DEPLOYMENT")

		for _, deploy := range deplist.Items {
			if isSystemNamespace(deploy.Namespace) {
				continue
			}
			observed++

			desired := int32(1)
			if deploy.Spec.Replicas != nil {
				desired = *deploy.Spec.Replicas
			}

			// ---------- kubectl-style ----------
			printKubectl(deployLine(deploy))

			// ---------- ORIGINAL deep logs ----------
			l.V(1).Info(
				"deployment details",
				"name", deploy.Name,
				"namespace", deploy.Namespace,
				"replicas.desired", desired,
				"replicas.updated", deploy.Status.UpdatedReplicas,
				"replicas.available", deploy.Status.AvailableReplicas,
				"replicas.unavailable", deploy.Status.UnavailableReplicas,
				"generation", deploy.Generation,
				"observedGeneration", deploy.Status.ObservedGeneration,
				"conditions", deploy.Status.Conditions,
			)
		}
	}

	// ---------------- REPLICASETS ----------------
	if shouldLogReplicaSets(&logger) {
		var rslist appsv1.ReplicaSetList
		if err := r.List(ctx, &rslist, opts...); err != nil {
			return ctrl.Result{}, err
		}

		l.V(1).Info("replicasets observed", "count", len(rslist.Items))
		printKubectl("REPLICASETS")
		for _, rs := range rslist.Items {
			if isSystemNamespace(rs.Namespace) {
				continue
			}
			observed++

			desired := int32(1)
			if rs.Spec.Replicas != nil {
				desired = *rs.Spec.Replicas
			}

			// ---------- kubectl-style ----------
			printKubectl(rsLine(rs))

			// ---------- ORIGINAL deep logs ----------
			l.V(1).Info(
				"replicaset details",
				"name", rs.Name,
				"namespace", rs.Namespace,
				"replicas.desired", desired,
				"replicas.current", rs.Status.Replicas,
				"replicas.ready", rs.Status.ReadyReplicas,
				"replicas.available", rs.Status.AvailableReplicas,
				"generation", rs.Generation,
				"observedGeneration", rs.Status.ObservedGeneration,
				"ownerRefs", rs.OwnerReferences,
			)
		}

	}

	now := metav1.Now()

	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latest := &loggerv1.Logger{}
		if err := r.Get(ctx, req.NamespacedName, latest); err != nil {
			return err
		}

		latest.Status.ObservedResources = observed
		latest.Status.LastRunTime = &now
		latest.Status.LastError = ""

		return r.Status().Update(ctx, latest)
	}); err != nil {
		return ctrl.Result{}, err
	}

	l.V(1).Info(
		"reconcile complete",
		"observedResources", observed,
		"requeueAfter", requeueAfter.String(),
	)

	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LoggerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(
			&loggerv1.Logger{},
			builder.WithPredicates(
				predicate.ResourceVersionChangedPredicate{},
			),
		).
		Watches(
			&corev1.Pod{},
			handler.EnqueueRequestsFromMapFunc(r.loggerRequestsForPod),
		).Watches(
		&appsv1.Deployment{},
		handler.EnqueueRequestsFromMapFunc(r.loggerRequestsForDeployment),
	).
		Named("logger").
		Complete(r)
}
