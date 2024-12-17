/*
Copyright 2022.

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

// Example code used for influence: https://github.com/Azure/azure-databricks-operator/blob/0f722a710fea06b86ecdccd9455336ca712bf775/controllers/dcluster_controller_test.go

package checkly

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	checklyv1alpha1 "github.com/checkly/checkly-operator/api/checkly/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("ApiCheck Controller", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		timeout  = time.Second * 10
		duration = time.Second * 10
		interval = time.Millisecond * 250
	)

	BeforeEach(func() {
		// Add any setup steps that needs to be executed before each test
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
	})

	// Add Tests for OpenAPI validation (or additonal CRD features) specified in
	// your API definition.
	// Avoid adding tests for vanilla CRUD operations because they would
	// test Kubernetes API server, which isn't the goal here.
	Context("ApiCheck", func() {
		It("Full reconciliation with body and body type", func() {

			key := types.NamespacedName{
				Name:      "test-apicheck",
				Namespace: "default",
			}

			groupKey := types.NamespacedName{
				Name: "test-apicheck-group",
			}

			group := &checklyv1alpha1.Group{
				ObjectMeta: metav1.ObjectMeta{
					Name: groupKey.Name,
				},
			}

			apiCheck := &checklyv1alpha1.ApiCheck{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
				Spec: checklyv1alpha1.ApiCheckSpec{
					Endpoint: "http://bar.baz/quoz",
					Group:    groupKey.Name,
					Muted:    true,
					Method:   "POST",
					Body:     `{"jsonrpc":"2.0","method":"eth_syncing","params":[],"id":1}`,
					BodyType: "json",
					Assertions: []checklyv1alpha1.Assertion{
						{
							Source:     "STATUS_CODE",
							Comparison: "EQUALS",
							Target:     "200",
						},
						{
							Source:     "JSON_BODY",
							Property:   "$.status",
							Comparison: "NOT_NULL",
						},
					},
				},
			}

			// Create
			Expect(k8sClient.Create(context.Background(), group)).Should(Succeed())
			Expect(k8sClient.Create(context.Background(), apiCheck)).Should(Succeed())

			By("Expecting submitted")
			Eventually(func() bool {
				f := &checklyv1alpha1.ApiCheck{}
				err := k8sClient.Get(context.Background(), key, f)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			// Status.ID should be present
			By("Expecting group ID, method, body, body type, and assertions")
			Eventually(func() bool {
				f := &checklyv1alpha1.ApiCheck{}
				err := k8sClient.Get(context.Background(), key, f)
				if f.Status.ID != "2" && err != nil {
					return false
				}

				if f.Spec.Muted != true {
					return false
				}

				if f.Spec.Method != "POST" {
					return false
				}

				if f.Spec.Body != `{"jsonrpc":"2.0","method":"eth_syncing","params":[],"id":1}` {
					return false
				}

				if f.Spec.BodyType != "json" {
					return false
				}

				if len(f.Spec.Assertions) != 2 {
					return false
				}

				return true
			}, timeout, interval).Should(BeTrue())

			// Finalizer should be present
			By("Expecting finalizer")
			Eventually(func() bool {
				f := &checklyv1alpha1.ApiCheck{}
				err := k8sClient.Get(context.Background(), key, f)
				if err != nil {
					return false
				}

				for _, finalizer := range f.Finalizers {
					Expect(finalizer).To(Equal("testing.domain.tld/finalizer"), "Finalizer should match")
				}

				return true
			}, timeout, interval).Should(BeTrue())

			// Delete
			Expect(k8sClient.Delete(context.Background(), group)).Should(Succeed())

			By("Expecting to delete successfully")
			Eventually(func() error {
				f := &checklyv1alpha1.ApiCheck{}
				k8sClient.Get(context.Background(), key, f)
				return k8sClient.Delete(context.Background(), f)
			}, timeout, interval).Should(Succeed())

			By("Expecting delete to finish")
			Eventually(func() error {
				f := &checklyv1alpha1.ApiCheck{}
				return k8sClient.Get(context.Background(), key, f)
			}, timeout, interval).ShouldNot(Succeed())
		})
	})
})
