/*
Copyright 2023. projectsveltos.io. All rights reserved.

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

package controller_test

import (
	"context"
	"sync"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"gianlucam76/claudie-sveltos-integration/internal/controller"

	libsveltosv1alpha1 "github.com/projectsveltos/libsveltos/api/v1alpha1"
)

var _ = Describe("SecretReconciler", func() {
	It("isSveltosClusterForClaudie returns true when SveltosCluster is created for a Claudie Secret", func() {
		sveltosCluster := &libsveltosv1alpha1.SveltosCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: randomString(),
				Name:      randomString(),
			},
		}

		Expect(controller.IsSveltosClusterForClaudie(sveltosCluster)).To(BeFalse())

		sveltosCluster.Annotations = map[string]string{}
		Expect(controller.IsSveltosClusterForClaudie(sveltosCluster)).To(BeFalse())

		sveltosCluster.Annotations[randomString()] = randomString()
		Expect(controller.IsSveltosClusterForClaudie(sveltosCluster)).To(BeFalse())

		sveltosCluster.Annotations[controller.SveltosClusterClaudieAnnotation] = "ok"
		Expect(controller.IsSveltosClusterForClaudie(sveltosCluster)).To(BeTrue())
	})

	It("isClaudieSecretRemoved returns true when Secret is not existing anymore", func() {
		c := fake.NewClientBuilder().WithScheme(scheme).Build()

		claudieSecret := &types.NamespacedName{Namespace: randomString(), Name: randomString()}
		Expect(controller.IsClaudieSecretRemoved(context.TODO(), c, claudieSecret)).To(BeTrue())

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: claudieSecret.Namespace,
				Name:      claudieSecret.Name,
			},
		}

		Expect(c.Create(context.TODO(), secret)).To(Succeed())
		Expect(controller.IsClaudieSecretRemoved(context.TODO(), c, claudieSecret)).To(BeFalse())
	})

	It("getClaudieSecret returns secret", func() {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: randomString(),
				Name:      randomString(),
			},
		}

		sveltosCluster := &libsveltosv1alpha1.SveltosCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: secret.Namespace,
				Name:      randomString(),
				OwnerReferences: []metav1.OwnerReference{
					{
						Kind:       "Secret",
						APIVersion: "v1",
						Name:       secret.Name,
					},
				},
			},
		}

		secretInfo := controller.GetClaudieSecret(sveltosCluster)
		Expect(secretInfo).ToNot(BeNil())
		Expect(secretInfo.Namespace).To(Equal(secret.Namespace))
		Expect(secretInfo.Name).To(Equal(secret.Name))
	})

	It("shouldReconcileSecret returns true for Claudie secrets", func() {
		c := fake.NewClientBuilder().WithScheme(scheme).Build()
		reconciler := getSecretReconciler(c)

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: randomString(),
				Name:      randomString(),
			},
		}

		Expect(controller.ShouldReconcileSecret(reconciler, secret)).To(BeFalse())

		secret.Labels = map[string]string{
			randomString(): randomString(),
		}
		Expect(controller.ShouldReconcileSecret(reconciler, secret)).To(BeFalse())

		secret.Labels[controller.ClaudieLabel] = randomString()
		Expect(controller.ShouldReconcileSecret(reconciler, secret)).To(BeFalse())

		secret.Labels[controller.ClaudieKubeconfig] = randomString()
		Expect(controller.ShouldReconcileSecret(reconciler, secret)).To(BeFalse())

		secret.Labels[controller.ClaudieCluster] = randomString()
		Expect(controller.ShouldReconcileSecret(reconciler, secret)).To(BeTrue())
	})

	It("getSveltosClusterNamespace returns secret namespace", func() {
		c := fake.NewClientBuilder().WithScheme(scheme).Build()
		reconciler := getSecretReconciler(c)

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: randomString(),
				Name:      randomString(),
			},
		}

		Expect(controller.GetSveltosClusterNamespace(reconciler, secret)).To(Equal(secret.Namespace))
	})

	It("cleanSveltosCluster deletes SveltosCluster", func() {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: randomString(),
				Name:      randomString(),
			},
		}

		sveltosCluster := &libsveltosv1alpha1.SveltosCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: secret.Namespace,
				Name:      randomString(),
				OwnerReferences: []metav1.OwnerReference{
					{
						Kind:       "Secret",
						APIVersion: "v1",
						Name:       secret.Name,
					},
				},
			},
		}

		initObjects := []client.Object{
			secret, sveltosCluster,
		}

		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjects...).Build()
		reconciler := getSecretReconciler(c)

		secretRef := reconcile.Request{
			NamespacedName: types.NamespacedName{Namespace: secret.Namespace, Name: secret.Name},
		}

		reconciler.SecretToCluster[secretRef.NamespacedName] = types.NamespacedName{
			Namespace: sveltosCluster.Namespace, Name: sveltosCluster.Name,
		}

		Expect(controller.CleanSveltosCluster(reconciler, context.TODO(), secretRef, logr.Logger{})).To(BeNil())

		currentSveltosCluster := &libsveltosv1alpha1.SveltosCluster{}
		err := c.Get(context.TODO(),
			types.NamespacedName{Namespace: sveltosCluster.Namespace, Name: sveltosCluster.Name},
			currentSveltosCluster)
		Expect(err).ToNot(BeNil())
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
	})

	It("addOwnerReference add secret as SveltosCluster's OwnerReference", func() {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: randomString(),
				Name:      randomString(),
			},
		}
		Expect(addTypeInformationToObject(scheme, secret)).To(Succeed())

		sveltosCluster := &libsveltosv1alpha1.SveltosCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: secret.Namespace,
				Name:      randomString(),
			},
		}

		initObjects := []client.Object{
			secret, sveltosCluster,
		}

		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjects...).Build()
		reconciler := getSecretReconciler(c)

		controller.AddOwnerReference(reconciler, sveltosCluster, secret)

		Expect(sveltosCluster.OwnerReferences).ToNot(BeNil())
		Expect(len(sveltosCluster.OwnerReferences)).To(Equal(1))
		Expect(sveltosCluster.OwnerReferences[0].Kind).To(Equal("Secret"))
		Expect(sveltosCluster.OwnerReferences[0].Name).To(Equal(secret.Name))
	})

	It("addAnnotation adds claudie annotation to SveltosCluster", func() {
		c := fake.NewClientBuilder().WithScheme(scheme).Build()
		reconciler := getSecretReconciler(c)

		sveltosCluster := &libsveltosv1alpha1.SveltosCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: randomString(),
				Name:      randomString(),
			},
		}

		controller.AddAnnotation(reconciler, sveltosCluster)
		Expect(sveltosCluster.Annotations).ToNot(BeNil())
		Expect(sveltosCluster.Annotations[controller.SveltosClusterClaudieAnnotation]).ToNot(BeEmpty())
	})

	It("createSveltosCluster creates a SveltosCluster for a Claudie secret", func() {
		c := fake.NewClientBuilder().WithScheme(scheme).Build()
		reconciler := getSecretReconciler(c)

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: randomString(),
				Name:      randomString(),
				Labels: map[string]string{
					controller.ClaudieLabel:      "claudie",
					controller.ClaudieKubeconfig: "kubeconfig",
					controller.ClaudieCluster:    randomString(),
				},
			},
		}

		Expect(controller.CreateSveltosCluster(reconciler, context.TODO(), secret, logr.Logger{})).To(Succeed())

		currentSveltosClusters := &libsveltosv1alpha1.SveltosClusterList{}
		Expect(c.List(context.TODO(), currentSveltosClusters)).To(Succeed())
		Expect(len(currentSveltosClusters.Items)).To(Equal(1))
		Expect(currentSveltosClusters.Items[0].Namespace).To(Equal(secret.Namespace))
		Expect(currentSveltosClusters.Items[0].Spec.KubeconfigName).To(Equal(secret.Name))
		Expect(currentSveltosClusters.Items[0].Annotations).ToNot(BeNil())
		Expect(currentSveltosClusters.Items[0].Annotations[controller.SveltosClusterClaudieAnnotation]).ToNot(BeEmpty())
		Expect(currentSveltosClusters.Items[0].OwnerReferences).ToNot(BeNil())
		Expect(len(currentSveltosClusters.Items[0].OwnerReferences)).To(Equal(1))
		Expect(currentSveltosClusters.Items[0].OwnerReferences[0].Name).To(Equal(secret.Name))
	})
})

func getSecretReconciler(c client.Client) *controller.SecretReconciler {
	return &controller.SecretReconciler{
		Client:          c,
		Mux:             sync.Mutex{},
		SecretToCluster: map[types.NamespacedName]types.NamespacedName{},
	}
}
