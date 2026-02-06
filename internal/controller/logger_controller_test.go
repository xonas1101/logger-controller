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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	loggerv1 "github.com/xonas1101/logger-controller/api/v1"
)

func testPod(name, namespace string) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"test": "logger",
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
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
			Labels: map[string]string{
				"test": "logger",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": name,
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": name,
						"test": "logger",
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
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
var _ = Describe("Logger Controller", func() {
	Context("When reconciling a logger", func() {
		const loggerName = "test-logger"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      loggerName,
			Namespace: "default", // TODO(user):Modify as needed
		}
    watchedns := &v1.Namespace{
      ObjectMeta: metav1.ObjectMeta{
      Name: "watchedns",
    	},
  	}

		unwatchedns := &v1.Namespace{
      ObjectMeta: metav1.ObjectMeta{
      Name: "unwatchedns",
    	},
  	}

		BeforeEach(func() {
			By("creating namespaces used by the Logger scope")
			err := k8sClient.Get(ctx, types.NamespacedName{Name: watchedns.Name}, &v1.Namespace{})
      if err != nil {
				Expect(errors.IsNotFound(err)).To(BeTrue())
				err = k8sClient.Create(ctx, watchedns)
				Expect(err).NotTo(HaveOccurred())
			}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: unwatchedns.Name}, &v1.Namespace{})
      if err != nil {
				Expect(errors.IsNotFound(err)).To(BeTrue())
				err = k8sClient.Create(ctx, unwatchedns)
				Expect(err).NotTo(HaveOccurred())
			}
		})


		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the logger instance.
			logger := &loggerv1.Logger{}
			err := k8sClient.Get(ctx, typeNamespacedName, logger)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific logger instance Logger")
			Expect(k8sClient.Delete(ctx, logger)).To(Succeed())
		})
		It("watches only the specified namespace when one namespace is provided", func() {
				logger := &loggerv1.Logger{
					ObjectMeta: metav1.ObjectMeta{
						Name:      loggerName,
						Namespace: watchedns.Name,
					},
					Spec: loggerv1.LoggerSpec{
						Scope: loggerv1.ScopeSpec{
							Type:       "Namespace",
							Namespace: watchedns.Name,
						},
					},
				}

	Expect(k8sClient.Create(ctx, logger)).To(Succeed())
			By("Reconciling the created logger")
			controllerReconciler := &LoggerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})
})
