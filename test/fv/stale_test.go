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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	libsveltosv1alpha1 "github.com/projectsveltos/libsveltos/api/v1alpha1"
)

var _ = Describe("Stale SveltosCluster are removed", func() {
	It("Stale SveltosCluster owned by non-existing Claudie Secret are removed", Label("FV"), func() {
		sveltosCluster := &libsveltosv1alpha1.SveltosCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      randomString(),
				Annotations: map[string]string{
					"projectsveltos.io/claudie": "ok",
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						Kind:       "Secret",
						Name:       randomString(),
						APIVersion: "v1",
						UID:        types.UID(randomString()),
					},
				},
			},
		}

		Byf("Creating a stale (Claudie secret does not exist) SveltosCluster %s/%s",
			sveltosCluster.Namespace, sveltosCluster.Name)
		Expect(k8sClient.Create(context.TODO(), sveltosCluster)).To(Succeed())

		Byf("Verifying SveltosCluster is gone")
		Eventually(func() bool {
			currentSveltosCluster := &libsveltosv1alpha1.SveltosCluster{}
			err := k8sClient.Get(context.TODO(),
				types.NamespacedName{Namespace: sveltosCluster.Namespace, Name: sveltosCluster.Name},
				currentSveltosCluster)
			if err == nil {
				return false
			}
			return apierrors.IsNotFound(err)
		}, timeout, pollingInterval).Should(BeTrue())
	})
})
