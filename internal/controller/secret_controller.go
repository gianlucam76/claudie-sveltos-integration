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

package controller

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	libsveltosv1alpha1 "github.com/projectsveltos/libsveltos/api/v1alpha1"
	logs "github.com/projectsveltos/libsveltos/lib/logsettings"
)

// SecretReconciler reconciles a Secret object
type SecretReconciler struct {
	client.Client
	Scheme               *runtime.Scheme
	ConcurrentReconciles int

	// use a Mutex to update Map as MaxConcurrentReconciles is higher than one
	Mux sync.Mutex

	// When a cluster is created with Claudie, a Secret is created
	// by Claudie containing the Kubeconfig to access such cluster.
	// This controller automatically creates a SveltosCluster for each Claudie cluster,
	// i.e when a Claudie Secret containing a cluster kubeconfig is detected, a SveltosCluster
	// instance is created.
	// This map contains the Claudie secret to SveltosCluster association
	SecretToCluster map[types.NamespacedName]types.NamespacedName
}

const (
	claudieLabel      = "app.kubernetes.io/part-of"
	claudieKubeconfig = "claudie.io/output"
	claudieCluster    = "claudie.io/cluster"

	sveltosClusterClaudieAnnotation = "projectsveltos.io/claudie"
)

const (
	// normalRequeueAfter is how long to wait before reconciling again if a failure happened
	normalRequeueAfter = 10 * time.Second
)

//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
//+kubebuilder:rbac:groups=lib.projectsveltos.io,resources=sveltosclusters,verbs=get;list;watch;update;patch;create;delete

func (r *SecretReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)
	logger.V(logs.LogInfo).Info("Reconciling")

	// Fecth the Secret instance
	secret := &corev1.Secret{}
	if err := r.Get(ctx, req.NamespacedName, secret); err != nil {
		if apierrors.IsNotFound(err) {
			err = r.cleanSveltosCluster(ctx, req, logger)
			if err != nil {
				logger.V(logs.LogInfo).Info(fmt.Sprintf("failed to reconcile: %v", err))
				return reconcile.Result{Requeue: true, RequeueAfter: normalRequeueAfter}, nil
			}
		}
		logger.Error(err, "Failed to fetch Secret")
		return reconcile.Result{}, errors.Wrapf(err, "Failed to fetch Secret %s", req.NamespacedName)
	}

	// Handle deleted cluster
	if !secret.DeletionTimestamp.IsZero() {
		err := r.cleanSveltosCluster(ctx, req, logger)
		if err != nil {
			logger.V(logs.LogInfo).Info(fmt.Sprintf("failed to reconcile: %v", err))
			return reconcile.Result{Requeue: true, RequeueAfter: normalRequeueAfter}, nil
		}
	}

	if !r.shouldReconcileSecret(secret) {
		return reconcile.Result{}, nil
	}

	err := r.createSveltosCluster(ctx, secret, logger)
	if err != nil {
		logger.V(logs.LogInfo).Info(fmt.Sprintf("failed to reconcile: %v", err))
		return reconcile.Result{Requeue: true, RequeueAfter: normalRequeueAfter}, nil
	}

	return reconcile.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SecretReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, logger logr.Logger) error {
	go cleanStaleSveltosCluster(ctx, mgr.GetClient(), logger)

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: r.ConcurrentReconciles,
		}).
		Complete(r)
}

// shouldReconcileSecret looks at Secret labels and return whether reconciler
// should process this one or not.
// Only Claudie secrets containing a cluster Kubeconfig are reconciled.
func (r *SecretReconciler) shouldReconcileSecret(secret *corev1.Secret) bool {
	if secret.Labels == nil {
		return false
	}

	if _, ok := secret.Labels[claudieLabel]; !ok {
		return false
	}

	if _, ok := secret.Labels[claudieKubeconfig]; !ok {
		return false
	}

	if _, ok := secret.Labels[claudieCluster]; !ok {
		return false
	}

	return true
}

func (r *SecretReconciler) getSveltosClusterName(secret *corev1.Secret) string {
	return secret.Labels[claudieCluster]
}

func (r *SecretReconciler) getSveltosClusterNamespace(secret *corev1.Secret) string {
	// SveltosCluster and Secret must be in same namespace. Secret is added as OwnerReference
	// for SveltosCluster.
	return secret.Namespace
}

// cleanSveltosCluster removes SveltosCluster (if any exists) for a given secret
func (r *SecretReconciler) cleanSveltosCluster(ctx context.Context, secretRef ctrl.Request,
	logger logr.Logger) error {

	r.Mux.Lock()
	defer r.Mux.Unlock()

	secretKey := types.NamespacedName{Namespace: secretRef.Namespace, Name: secretRef.Name}
	sveltosClusterInfo, ok := r.SecretToCluster[secretKey]
	if !ok {
		return nil
	}

	logger = logger.WithValues("secret", fmt.Sprintf("%s/%s", secretKey.Namespace, secretKey.Name))
	logger.V(logs.LogInfo).Info("removing SveltosCluster for Secret")

	sveltosCluster := &libsveltosv1alpha1.SveltosCluster{}
	err := r.Get(ctx, sveltosClusterInfo, sveltosCluster)
	if err != nil {
		if apierrors.IsNotFound(err) {
			delete(r.SecretToCluster, secretKey)
			return nil
		}

		return err
	}

	err = r.Delete(ctx, sveltosCluster)
	if err != nil {
		return err
	}

	delete(r.SecretToCluster, secretKey)
	return nil
}

// createSveltosCluster creates, if not existing already, a SveltosCluster for a Claudie Secret containing
// kubeconfig to acces kubernetes cluster.
// Secret is added as OwnerReference.
// If SveltosCluster already exists, it gets updated.
func (r *SecretReconciler) createSveltosCluster(ctx context.Context, secret *corev1.Secret, logger logr.Logger) error {
	logger = logger.WithValues("secret", fmt.Sprintf("%s/%s", secret.Namespace, secret.Name))
	logger.V(logs.LogInfo).Info("reconciling secret")

	sveltosClusterNamespace := r.getSveltosClusterNamespace(secret)
	sveltosClusterName := r.getSveltosClusterName(secret)

	r.updateSecretToClusterMap(secret, sveltosClusterNamespace, sveltosClusterName)

	sveltosCluster := &libsveltosv1alpha1.SveltosCluster{}
	err := r.Get(ctx,
		types.NamespacedName{Namespace: sveltosClusterNamespace, Name: sveltosClusterName},
		sveltosCluster)
	if err != nil {
		if apierrors.IsNotFound(err) {
			sveltosCluster.Namespace = sveltosClusterNamespace
			sveltosCluster.Name = sveltosClusterName
			sveltosCluster.Spec.KubeconfigName = secret.Name
			// SveltosCluster labels are used by Projectsveltos controller to decide
			// which add-ons/applications to deploy. So we only set OwnerReference and
			// Annotations and do not add any labels. Labels are managed by users only.
			r.addAnnotation(sveltosCluster)
			r.addOwnerReference(sveltosCluster, secret)
			return r.Create(ctx, sveltosCluster)
		}

		return err
	}

	r.addAnnotation(sveltosCluster)
	r.addOwnerReference(sveltosCluster, secret)
	return r.Update(ctx, sveltosCluster)
}

// addAnnotation adds an annotation to SveltosCluster indicating it was created for a Claudie Secret
func (r *SecretReconciler) addAnnotation(sveltosCluster *libsveltosv1alpha1.SveltosCluster) {
	if sveltosCluster.Annotations == nil {
		sveltosCluster.Annotations = make(map[string]string)
	}

	sveltosCluster.Annotations[sveltosClusterClaudieAnnotation] = "ok"
}

// addOwnerReference adds secret as owner for sveltosCluster
// When cleaning up, a SveltosCluster can be removed only if corresponding Secret is not present anymore.
func (r *SecretReconciler) addOwnerReference(sveltosCluster, secret client.Object) {
	onwerReferences := sveltosCluster.GetOwnerReferences()
	if onwerReferences == nil {
		onwerReferences = make([]metav1.OwnerReference, 0)
	}

	for i := range onwerReferences {
		ref := &onwerReferences[i]
		if ref.Kind == secret.GetObjectKind().GroupVersionKind().Kind &&
			ref.Name == secret.GetName() {

			return
		}
	}

	apiVersion, kind := secret.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()

	onwerReferences = append(onwerReferences,
		metav1.OwnerReference{
			APIVersion: apiVersion,
			Kind:       kind,
			Name:       secret.GetName(),
			UID:        secret.GetUID(),
		},
	)

	sveltosCluster.SetOwnerReferences(onwerReferences)
}

// updateSecretToClusterMap updates internal map that keeps track of SveltosCluster for a given Secret
func (r *SecretReconciler) updateSecretToClusterMap(secret *corev1.Secret, sveltosClusterNamespace, sveltosClusterName string) {
	secretRef := types.NamespacedName{Namespace: secret.Namespace, Name: secret.Name}

	r.Mux.Lock()
	defer r.Mux.Unlock()

	r.SecretToCluster[secretRef] = types.NamespacedName{Namespace: sveltosClusterNamespace, Name: sveltosClusterName}
}

// cleanStaleSveltosCluster is a background task that fetches existing SveltosClusters.
// If Owned by a Claudie secret that does not exist anymore, SveltosCluster is deleted.
func cleanStaleSveltosCluster(ctx context.Context, c client.Client, logger logr.Logger) {
	for {
		const sleepTime = 2 * time.Minute
		time.Sleep(sleepTime)

		sveltosClusters := &libsveltosv1alpha1.SveltosClusterList{}
		err := c.List(context.TODO(), sveltosClusters)
		if err != nil {
			continue
		}

		for i := range sveltosClusters.Items {
			sveltosCluster := &sveltosClusters.Items[i]

			// ignore SveltosCluster if marked for deletion
			if !sveltosCluster.DeletionTimestamp.IsZero() {
				continue
			}

			// ignore SveltosCluster if not created for a Claudie Secret
			if !isSveltosClusterForClaudie(sveltosCluster) {
				continue
			}

			claudieSecret := getClaudieSecret(sveltosCluster)
			if claudieSecret == nil {
				logger.V(logs.LogInfo).Info(
					fmt.Sprintf("found SveltosCluster %s/%s with no Claudie reference",
						sveltosCluster.Namespace, sveltosCluster.Name))
			}

			if !isClaudieSecretRemoved(ctx, c, claudieSecret) {
				continue
			}

			err = c.Delete(ctx, sveltosCluster)
			if err != nil {
				logger.V(logs.LogInfo).Info(
					fmt.Sprintf("failed to delete sveltosCluster %s/%s: %v",
						sveltosCluster.Namespace, sveltosCluster.Name, err))
			}
		}
	}
}

// isSveltosClusterForClaudie returns true if SveltosCluster was created for a Claudie
// secret
func isSveltosClusterForClaudie(sveltosCluster *libsveltosv1alpha1.SveltosCluster) bool {
	if sveltosCluster.Annotations == nil {
		return false
	}

	_, ok := sveltosCluster.Annotations[sveltosClusterClaudieAnnotation]
	return ok
}

func getClaudieSecret(sveltosCluster *libsveltosv1alpha1.SveltosCluster) *types.NamespacedName {
	for i := range sveltosCluster.OwnerReferences {
		ref := &sveltosCluster.OwnerReferences[i]
		if ref.Kind == "Secret" {
			return &types.NamespacedName{
				Name:      ref.Name,
				Namespace: sveltosCluster.Namespace,
			}
		}
	}

	return nil
}

func isClaudieSecretRemoved(ctx context.Context, c client.Client, claudieSecret *types.NamespacedName) bool {
	secret := &corev1.Secret{}
	err := c.Get(ctx, *claudieSecret, secret)
	if err != nil {
		return apierrors.IsNotFound(err)
	}

	return !secret.DeletionTimestamp.IsZero()
}
