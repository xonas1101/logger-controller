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
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	loggerv1 "github.com/xonas1101/logger-controller/api/v1"
)

var _ = Describe("Logger Controller", func() {
	Context("When reconciling a logger", func() {
		const loggerName = "test-logger"

		const (
			watchedns = "logger-watched"
			unwatchedns = "logger-unwatched"
			crName = "logger-sample"
		)

		ctx := context.Background()

		logger := &loggerv1.Logger{}
		typeNamespacedName := types.NamespacedName{
			Name:      loggerName,
			Namespace: "default", // TODO(user):Modify as needed
		}

		BeforeEach(func() {
			By("creating namespaces used by the Logger scope")

			for _, ns := range []string{watchedns, unwatchedns} {
				err := k8sClient.Create(ctx, &v1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: ns,
					},
				})

				if err != nil && !errors.IsAlreadyExists(err) {
					Expect(err).NotTo(HaveOccurred())
				}
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
		It("should successfully reconcile the logger", func() {
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
