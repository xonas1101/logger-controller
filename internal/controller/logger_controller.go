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

func (r *LoggerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var observed int32 = 0
	l := logf.FromContext(ctx).WithValues(
		"logger", req.String(),
	)
	l.Info("logger reconcile")

	var logger loggerv1.Logger
	if err := r.Get(ctx, req.NamespacedName, &logger); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	requeueAfter := 30 * time.Second
	if logger.Spec.Trigger != "" {
		d, err := time.ParseDuration(logger.Spec.Trigger)
		if err != nil {
			l.Error(err, "invalid spec.trigger, must be a Go duration (e.g. 30s, 5m)")
			logger.Status.LastError = err.Error()
			return ctrl.Result{}, err
		}
		requeueAfter = d
	}

	l.V(2).Info("reconcile triggered", "spec", logger.Spec)

	if !shouldLogPods(&logger) && !shouldLogDeployments(&logger) {
		l.V(2).Info("pods/deployments not enabled via spec.resources")
		return ctrl.Result{}, nil
	}

	opts := listOptionsFromScope(&logger)
	var podList corev1.PodList

	if err := r.List(ctx, &podList, opts...); err != nil {
		l.V(2).Error(err, "failed to list pods")
		return ctrl.Result{}, err
	}

	l.V(2).Info("pods observed", "count", len(podList.Items))

	for _, pod := range podList.Items {
		if isSystemNamespace(pod.Namespace) {
			continue
		}
		observed++
		l.Info(
			"pod observed",
			"namespace", pod.Namespace,
			"name", pod.Name,
			"node", pod.Spec.NodeName,
			"phase", pod.Status.Phase,
		)
	}

	if shouldLogDeployments(&logger) {
		var deployList appsv1.DeploymentList
		if err := r.List(ctx, &deployList, opts...); err != nil {
			l.Error(err, "failed to list deployments")
			return ctrl.Result{}, err
		}

		l.V(2).Info("deployments observed", "count", len(deployList.Items))

		for _, deploy := range deployList.Items {
			if isSystemNamespace(deploy.Namespace) {
				continue
			}
			desired := int32(1)
			if deploy.Spec.Replicas != nil {
				desired = *deploy.Spec.Replicas
			}
			observed++
			// use desired as a variable because desired is an optional field, which can return nil pointers
			l.Info(
				"deployment observed",
				"namespace", deploy.Namespace,
				"name", deploy.Name,
				"replicas.desired", desired,
				"replicas.updated", deploy.Status.UpdatedReplicas,
				"replicas.available", deploy.Status.AvailableReplicas,
				"replicas.unavailable", deploy.Status.UnavailableReplicas,
				"generation", deploy.Generation,
				"observedGeneration", deploy.Status.ObservedGeneration,
			)
		}
	}
	now := metav1.Now()

	logger.Status.ObservedResources = observed
	logger.Status.LastRunTime = &now
	logger.Status.LastError = ""

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

	l.Info("=== END OF THIS RECONCILE ===")
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
