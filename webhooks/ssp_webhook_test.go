/*


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

package webhooks

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	ssp "kubevirt.io/ssp-operator/api/v1beta2"
	"kubevirt.io/ssp-operator/internal"
)

var _ = Describe("SSP Validation", func() {
	var (
		client  client.Client
		objects = make([]runtime.Object, 0)

		validator admission.CustomValidator
		ctx       context.Context
	)

	JustBeforeEach(func() {
		scheme := runtime.NewScheme()
		// add our own scheme
		Expect(ssp.SchemeBuilder.AddToScheme(scheme)).To(Succeed())
		// add more schemes
		Expect(v1.AddToScheme(scheme)).To(Succeed())

		client = fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objects...).Build()

		validator = newSspValidator(client)
		ctx = context.Background()
	})

	Context("creating SSP CR", func() {
		const (
			templatesNamespace = "test-templates-ns"
		)

		BeforeEach(func() {
			objects = append(objects, &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:            templatesNamespace,
					ResourceVersion: "1",
				},
			})
		})

		AfterEach(func() {
			objects = make([]runtime.Object, 0)
		})

		Context("when one is already present", func() {
			BeforeEach(func() {
				// add an SSP CR to fake client
				objects = append(objects, &ssp.SSP{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "test-ssp",
						Namespace:       "test-ns",
						ResourceVersion: "1",
					},
					Spec: ssp.SSPSpec{
						CommonTemplates: ssp.CommonTemplates{
							Namespace: templatesNamespace,
						},
					},
				})
			})

			It("should be rejected", func() {
				ssp := &ssp.SSP{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-ssp2",
						Namespace: "test-ns2",
					},
					Spec: ssp.SSPSpec{
						CommonTemplates: ssp.CommonTemplates{
							Namespace: templatesNamespace,
						},
					},
				}
				err := validator.ValidateCreate(ctx, ssp)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("creation failed, an SSP CR already exists in namespace test-ns: test-ssp"))
			})
		})

		It("should fail if template namespace does not exist", func() {
			const nonexistingNamespace = "nonexisting-namespace"
			ssp := &ssp.SSP{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ssp",
					Namespace: "test-ns",
				},
				Spec: ssp.SSPSpec{
					CommonTemplates: ssp.CommonTemplates{
						Namespace: nonexistingNamespace,
					},
				},
			}
			err := validator.ValidateCreate(ctx, ssp)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("creation failed, the configured namespace for common templates does not exist: " + nonexistingNamespace))
		})
	})

	It("should allow update of commonTemplates.namespace", func() {
		oldSsp := &ssp.SSP{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-ssp",
				Namespace: "test-ns",
			},
			Spec: ssp.SSPSpec{
				CommonTemplates: ssp.CommonTemplates{
					Namespace: "old-ns",
				},
			},
		}

		newSsp := oldSsp.DeepCopy()
		newSsp.Spec.CommonTemplates.Namespace = "new-ns"

		err := validator.ValidateUpdate(ctx, oldSsp, newSsp)
		Expect(err).ToNot(HaveOccurred())
	})

	Context("DataImportCronTemplates", func() {
		const (
			templatesNamespace = "test-templates-ns"
		)

		var (
			oldSSP *ssp.SSP
			newSSP *ssp.SSP
		)

		BeforeEach(func() {
			objects = append(objects, &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:            templatesNamespace,
					ResourceVersion: "1",
				},
			})

			oldSSP = &ssp.SSP{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ssp",
					Namespace: "test-ns",
				},
				Spec: ssp.SSPSpec{
					CommonTemplates: ssp.CommonTemplates{
						Namespace: templatesNamespace,
						DataImportCronTemplates: []ssp.DataImportCronTemplate{
							{
								ObjectMeta: metav1.ObjectMeta{
									Namespace: internal.GoldenImagesNamespace,
								},
							},
						},
					},
				},
			}

			newSSP = oldSSP.DeepCopy()
		})

		AfterEach(func() {
			objects = make([]runtime.Object, 0)
		})

		It("should validate dataImportCronTemplates on create", func() {
			Expect(validator.ValidateCreate(ctx, newSSP)).To(HaveOccurred())
			newSSP.Spec.CommonTemplates.DataImportCronTemplates[0].Name = "test-name"
			Expect(validator.ValidateCreate(ctx, newSSP)).ToNot(HaveOccurred())
		})

		It("should validate dataImportCronTemplates on update", func() {
			Expect(validator.ValidateUpdate(ctx, oldSSP, newSSP)).To(HaveOccurred())
			newSSP.Spec.CommonTemplates.DataImportCronTemplates[0].Name = "test-name"
			Expect(validator.ValidateUpdate(ctx, oldSSP, newSSP)).ToNot(HaveOccurred())
		})
	})

	Context("CommonInstancetypes", func() {

		const (
			templatesNamespace = "test-templates-ns"
		)

		var sspObj *ssp.SSP

		BeforeEach(func() {
			objects = append(objects, &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:            templatesNamespace,
					ResourceVersion: "1",
				},
			})
			sspObj = &ssp.SSP{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ssp",
				},
				Spec: ssp.SSPSpec{
					CommonTemplates: ssp.CommonTemplates{
						Namespace: templatesNamespace,
					},
					CommonInstancetypes: &ssp.CommonInstancetypes{},
				},
			}
		})

		AfterEach(func() {
			objects = make([]runtime.Object, 0)
		})

		It("should reject URL without https:// or ssh://", func() {
			sspObj.Spec.CommonInstancetypes.URL = pointer.String("file://foo/bar")
			Expect(validator.ValidateCreate(ctx, sspObj)).ShouldNot(Succeed())
		})

		It("should reject URL without ?ref= or ?version=", func() {
			sspObj.Spec.CommonInstancetypes.URL = pointer.String("https://foo.com/bar")
			Expect(validator.ValidateCreate(ctx, sspObj)).ShouldNot(Succeed())
		})

		DescribeTable("should accept a valid remote kustomize target URL", func(url string) {
			sspObj.Spec.CommonInstancetypes.URL = pointer.String(url)
			Expect(validator.ValidateCreate(ctx, sspObj)).Should(Succeed())
		},
			Entry("https:// with ?ref=", "https://foo.com/bar?ref=1234"),
			Entry("https:// with ?target=", "https://foo.com/bar?version=1234"),
			Entry("ssh:// with ?ref=", "ssh://foo.com/bar?ref=1234"),
			Entry("ssh:// with ?target=", "ssh://foo.com/bar?version=1234"),
		)

		It("should accept when no URL is provided", func() {
			Expect(validator.ValidateCreate(ctx, sspObj)).Should(Succeed())
		})
	})
})

func TestWebhook(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "API Suite")
}
