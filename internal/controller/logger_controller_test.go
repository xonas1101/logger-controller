package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	loggerv1 "github.com/xonas1101/logger-controller/api/v1"
)

const (
	watchedNamespace   = "watchedns"
	unwatchedNamespace = "unwatchedns"
	loggerName         = "test-logger"
)

func testPod(name, namespace string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "pause",
					Image: "registry.k8s.io/pause:3.9",
				},
			},
		},
	}
}

func testDeployment(name, namespace string) *appsv1.Deployment {
	replicas := int32(1)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": name},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "pause",
							Image: "registry.k8s.io/pause:3.9",
						},
					},
				},
			},
		},
	}
}

func podIsLogged(ctx context.Context, name, namespace string) bool {
	pod := &corev1.Pod{}
	if err := k8sClient.Get(ctx,
		types.NamespacedName{Name: name, Namespace: namespace},
		pod,
	); err != nil {
		return false
	}

	if pod.Annotations == nil {
		return false
	}

	return pod.Annotations["logger/observed"] == "true"
}

func deploymentIsLogged(ctx context.Context, name, namespace string) bool {
	dep := &appsv1.Deployment{}
	if err := k8sClient.Get(ctx,
		types.NamespacedName{Name: name, Namespace: namespace},
		dep,
	); err != nil {
		return false
	}

	if dep.Annotations == nil {
		return false
	}

	return dep.Annotations["logger/observed"] == "true"
}

var _ = Describe("Logger Controller", func() {
	ctx := context.Background()

	loggerKey := types.NamespacedName{
		Name:      loggerName,
		Namespace: "default",
	}

	watchedNS := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: watchedNamespace},
	}

	unwatchedNS := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: unwatchedNamespace},
	}

	BeforeEach(func() {
		for _, ns := range []*corev1.Namespace{watchedNS, unwatchedNS} {
			err := k8sClient.Get(ctx, types.NamespacedName{Name: ns.Name}, &corev1.Namespace{})
			if err != nil {
				Expect(errors.IsNotFound(err)).To(BeTrue())
				Expect(k8sClient.Create(ctx, ns)).To(Succeed())
			}
		}
	})

	AfterEach(func() {
		logger := &loggerv1.Logger{}
		err := k8sClient.Get(ctx, loggerKey, logger)
		if err == nil {
			_ = k8sClient.Delete(ctx, logger)
		}
	})

	It("logs ONLY watched namespace resources when namespace scope is used", func() {
		logger := &loggerv1.Logger{
			ObjectMeta: metav1.ObjectMeta{
				Name:      loggerName,
				Namespace: "default",
			},
			Spec: loggerv1.LoggerSpec{
				Scope: loggerv1.ScopeSpec{
					Type:      "Namespace",
					Namespace: watchedNamespace,
				},
				Resources: []string{"pods", "deployments"},
			},
		}

		Expect(k8sClient.Create(ctx, testPod("pod-watched", watchedNamespace))).To(Succeed())
		Expect(k8sClient.Create(ctx, testPod("pod-unwatched", unwatchedNamespace))).To(Succeed())

		Expect(k8sClient.Create(ctx, testDeployment("dep-watched", watchedNamespace))).To(Succeed())
		Expect(k8sClient.Create(ctx, testDeployment("dep-unwatched", unwatchedNamespace))).To(Succeed())

		Expect(k8sClient.Create(ctx, logger)).To(Succeed())

		reconciler := &LoggerReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}

		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: loggerKey,
		})
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() bool {
			return podIsLogged(ctx, "pod-watched", watchedNamespace)
		}, 5*time.Second, 200*time.Millisecond).Should(BeTrue())

		Eventually(func() bool {
			return deploymentIsLogged(ctx, "dep-watched", watchedNamespace)
		}).Should(BeTrue())

		Consistently(func() bool {
			return podIsLogged(ctx, "pod-unwatched", unwatchedNamespace)
		}, 2*time.Second, 200*time.Millisecond).Should(BeFalse())

		Consistently(func() bool {
			return deploymentIsLogged(ctx, "dep-unwatched", unwatchedNamespace)
		}).Should(BeFalse())
	})

	It("logs ALL resources when cluster scope is used", func() {
		logger := &loggerv1.Logger{
			ObjectMeta: metav1.ObjectMeta{
				Name:      loggerName,
				Namespace: "default",
			},
			Spec: loggerv1.LoggerSpec{
				Scope: loggerv1.ScopeSpec{
					Type: "Cluster",
				},
				Resources: []string{"pods", "deployments"},
			},
		}

		Expect(k8sClient.Create(ctx, testPod("pod-a", watchedNamespace))).To(Succeed())
		Expect(k8sClient.Create(ctx, testPod("pod-b", unwatchedNamespace))).To(Succeed())

		Expect(k8sClient.Create(ctx, testDeployment("dep-a", watchedNamespace))).To(Succeed())
		Expect(k8sClient.Create(ctx, testDeployment("dep-b", unwatchedNamespace))).To(Succeed())

		Expect(k8sClient.Create(ctx, logger)).To(Succeed())

		reconciler := &LoggerReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}

		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: loggerKey,
		})
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() bool {
			return podIsLogged(ctx, "pod-a", watchedNamespace)
		}).Should(BeTrue())

		Eventually(func() bool {
			return podIsLogged(ctx, "pod-b", unwatchedNamespace)
		}).Should(BeTrue())

		Eventually(func() bool {
			return deploymentIsLogged(ctx, "dep-a", watchedNamespace)
		}).Should(BeTrue())

		Eventually(func() bool {
			return deploymentIsLogged(ctx, "dep-b", unwatchedNamespace)
		}).Should(BeTrue())
	})
})
