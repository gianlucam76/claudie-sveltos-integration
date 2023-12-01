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

package fv_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	libsveltosv1alpha1 "github.com/projectsveltos/libsveltos/api/v1alpha1"
)

var _ = Describe("Watch Claudie Secret and update SveltosCluster", func() {
	const (
		namePrefix = "lc-"
	)

	It("Manage SveltosCluster for Claudie Secret", Label("FV"), func() {
		secret := getClaudieSecret(namePrefix)
		Byf("Creating a Claudie secret %s/%s", secret.Namespace, secret.Name)
		Expect(k8sClient.Create(context.TODO(), secret)).To(Succeed())

		Byf("Verifying SveltosCluster is created")
		Eventually(func() bool {
			sveltosClusters := &libsveltosv1alpha1.SveltosClusterList{}
			err := k8sClient.List(context.TODO(), sveltosClusters)
			if err != nil {
				return false
			}
			for i := range sveltosClusters.Items {
				sveltosCluster := &sveltosClusters.Items[i]
				if isSecretOwner(sveltosCluster, secret) {
					return true
				}
			}
			return false
		}, timeout, pollingInterval).Should(BeTrue())

		Byf("Deleting Claudie secret %s/%s", secret.Namespace, secret.Name)
		currentSecret := &corev1.Secret{}
		Expect(k8sClient.Get(context.TODO(),
			types.NamespacedName{Namespace: secret.Namespace, Name: secret.Name},
			currentSecret),
		).To(Succeed())
		Expect(k8sClient.Delete(context.TODO(), currentSecret)).To(Succeed())

		Byf("Verifying SveltosCluster is gone")
		Eventually(func() bool {
			sveltosClusters := &libsveltosv1alpha1.SveltosClusterList{}
			err := k8sClient.List(context.TODO(), sveltosClusters)
			if err != nil {
				return false
			}
			for i := range sveltosClusters.Items {
				sveltosCluster := &sveltosClusters.Items[i]
				if isSecretOwner(sveltosCluster, secret) {
					return false
				}
			}
			return true
		}, timeout, pollingInterval).Should(BeTrue())
	})
})

func isSecretOwner(sveltosCluster *libsveltosv1alpha1.SveltosCluster, secret *corev1.Secret) bool {
	for i := range sveltosCluster.OwnerReferences {
		ownerRef := sveltosCluster.OwnerReferences[i]
		if ownerRef.Kind == "Secret" && ownerRef.Name == secret.Name {
			return true
		}
	}

	return false
}
