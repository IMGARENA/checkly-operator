package networking

import (
	"context"
	"fmt"
	"time"

	checklyv1alpha1 "github.com/checkly/checkly-operator/api/checkly/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Ingress Controller", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		timeout  = time.Second * 10
		duration = time.Second * 10
		interval = time.Millisecond * 250
	)

	BeforeEach(func() {
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
	})

	Context("Ingress", func() {

		// Test happy path
		It("full reconciliation", func() {

			testHost := "foo.bar"
			testPath := "baz"
			testGroup := "ingress-group"
			testSuccessCode := "200"

			apiCheckName := fmt.Sprintf("%s-%s-%s", "test-ingress", "foobar", testPath)

			group := &checklyv1alpha1.Group{
				ObjectMeta: metav1.ObjectMeta{
					Name: testGroup,
				},
				Spec: checklyv1alpha1.GroupSpec{
					Locations: []string{"eu-west-1"},
				},
			}

			ingressKey := types.NamespacedName{
				Name:      "test-ingress",
				Namespace: "default",
			}

			apiCheckKey := types.NamespacedName{
				Name:      apiCheckName,
				Namespace: "default",
			}

			annotation := make(map[string]string)
			annotation["testing.domain.tld/enabled"] = "true"
			annotation["testing.domain.tld/success"] = testSuccessCode
			annotation["testing.domain.tld/group"] = testGroup

			pathTypeImplementationSpecific := networkingv1.PathTypeImplementationSpecific

			var rules []networkingv1.IngressRule
			rules = append(rules, networkingv1.IngressRule{
				Host: testHost,
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{
							networkingv1.HTTPIngressPath{
								Path:     fmt.Sprintf("/%s", testPath),
								PathType: &pathTypeImplementationSpecific,
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: "test-service",
										Port: networkingv1.ServiceBackendPort{
											Number: 7777,
										},
									},
								},
							},
						},
					},
				},
			})

			ingress := &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:        ingressKey.Name,
					Namespace:   ingressKey.Namespace,
					Annotations: annotation,
				},
				Spec: networkingv1.IngressSpec{
					Rules: rules,
				},
			}

			// Create group
			Expect(k8sClient.Create(context.Background(), group)).Should(Succeed())

			// Create
			Expect(k8sClient.Create(context.Background(), ingress)).Should(Succeed())

			By("Expecting submitted")
			Eventually(func() bool {
				f := &networkingv1.Ingress{}
				err := k8sClient.Get(context.Background(), ingressKey, f)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Expecting ApiCheck and OwnerReference to exist")
			Eventually(func() bool {
				f := &checklyv1alpha1.ApiCheck{}
				err := k8sClient.Get(context.Background(), apiCheckKey, f)
				if err != nil {
					return false
				}

				if len(f.OwnerReferences) != 1 {
					return false
				}

				Expect(f.Spec.Endpoint == fmt.Sprintf("https://%s/%s", testHost, testPath)).To(BeTrue(), "Hosts should match.")
				Expect(f.Spec.Group).To(Equal(testGroup), "Group should match")
				Expect(f.Spec.Success).To(Equal(testSuccessCode), "Success code should match")
				Expect(f.Spec.Muted).To(Equal(true), "Mute should match")

				for _, o := range f.OwnerReferences {
					Expect(o.Name).To(Equal(ingressKey.Name), "OwnerReference should be equal")
				}

				return true
			}, timeout, interval).Should(BeTrue(), "Timed out waiting for success")

			// Delete
			By("Expecting to delete successfully")
			Eventually(func() error {
				f := &networkingv1.Ingress{}
				k8sClient.Get(context.Background(), ingressKey, f)
				return k8sClient.Delete(context.Background(), f)
			}, timeout, interval).Should(Succeed())

			By("Expecting delete to finish")
			Eventually(func() error {
				f := &networkingv1.Ingress{}
				return k8sClient.Get(context.Background(), ingressKey, f)
			}, timeout, interval).ShouldNot(Succeed())

			// Delete group
			Expect(k8sClient.Delete(context.Background(), group)).Should(Succeed(), "Group deletion should succeed")

		})

		// Testing failures
		It("Some failures", func() {
			testHost := "foo.bar"
			testPath := "baz"
			testSuccessCode := "200"

			key := types.NamespacedName{
				Name:      "test-fail-ingress",
				Namespace: "default",
			}

			annotation := make(map[string]string)
			annotation["testing.domain.tld/enabled"] = "false"
			annotation["testing.domain.tld/path"] = testPath
			annotation["testing.domain.tld/success"] = testSuccessCode
			annotation["testing.domain.tld/muted"] = "false"

			rules := make([]networkingv1.IngressRule, 0)
			rules = append(rules, networkingv1.IngressRule{
				Host: testHost,
			})

			ingress := &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:        key.Name,
					Namespace:   key.Namespace,
					Annotations: annotation,
				},
				Spec: networkingv1.IngressSpec{
					Rules: rules,
					DefaultBackend: &networkingv1.IngressBackend{
						Service: &networkingv1.IngressServiceBackend{
							Name: "test-service",
							Port: networkingv1.ServiceBackendPort{
								Number: 7777,
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(context.Background(), ingress)).Should(Succeed())

			// Delete
			By("Expecting to delete successfully")
			Eventually(func() error {
				f := &networkingv1.Ingress{}
				k8sClient.Get(context.Background(), key, f)
				return k8sClient.Delete(context.Background(), f)
			}, timeout, interval).Should(Succeed())

			By("Expecting delete to finish")
			Eventually(func() error {
				f := &networkingv1.Ingress{}
				return k8sClient.Get(context.Background(), key, f)
			}, timeout, interval).ShouldNot(Succeed())

		})
	})

})
