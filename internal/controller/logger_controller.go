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

	loggerv1 "github.com/xonas1101/logger-controller/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

func isSystemNamespace(ns string) bool {
    switch ns {
    case "kube-system", "kube-public", "kube-node-lease":
        return true
    }
    return false
}

func (r *LoggerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := logf.FromContext(ctx).WithValues(
			"logger", req.NamespacedName.String(),
	)
	l.Info("logger reconcile")

	var logger loggerv1.Logger
	if err := r.Get(ctx, req.NamespacedName, &logger); err != nil {
			return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	l.V(2).Info("reconcile triggered", "spec", logger.Spec)

	if !shouldLogPods(&logger) {
			l.V(2).Info("pods not enabled via spec.resources")
			return ctrl.Result{}, nil
	}

	var podList corev1.PodList
	opts := podListOptionsFromScope(&logger)

	if err := r.List(ctx, &podList, opts...); err != nil {
			l.V(2).Error(err, "failed to list pods")
			return ctrl.Result{}, err
	}

	l.V(2).Info("pods observed", "count", len(podList.Items))

	for _, pod := range podList.Items {
		if isSystemNamespace(pod.Namespace) {
        continue
    }
			l.Info(
					"pod observed",
					"namespace", pod.Namespace,
					"name", pod.Name,
					"node", pod.Spec.NodeName,
					"phase", pod.Status.Phase,
			)
	}
	l.Info("=== END OF THIS RECONCILE ===")
	return ctrl.Result{}, nil
}

func podListOptionsFromScope(logger *loggerv1.Logger) []client.ListOption {
    opts := []client.ListOption{}

    switch logger.Spec.Scope.Type {
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
    for _, r := range logger.Spec.Resources {
        if r == "pods" {
            return true
        }
    }
    return false
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
        ).
        Named("logger").
        Complete(r)
}