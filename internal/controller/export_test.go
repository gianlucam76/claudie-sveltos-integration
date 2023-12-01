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

const (
	SveltosClusterClaudieAnnotation = sveltosClusterClaudieAnnotation
)

var (
	IsSveltosClusterForClaudie = isSveltosClusterForClaudie
	GetClaudieSecret           = getClaudieSecret
	IsClaudieSecretRemoved     = isClaudieSecretRemoved
)

const (
	ClaudieLabel      = claudieLabel
	ClaudieKubeconfig = claudieKubeconfig
	ClaudieCluster    = claudieCluster
)

var (
	ShouldReconcileSecret      = (*SecretReconciler).shouldReconcileSecret
	GetSveltosClusterNamespace = (*SecretReconciler).getSveltosClusterNamespace
	CleanSveltosCluster        = (*SecretReconciler).cleanSveltosCluster
	AddOwnerReference          = (*SecretReconciler).addOwnerReference
	AddAnnotation              = (*SecretReconciler).addAnnotation
	CreateSveltosCluster       = (*SecretReconciler).createSveltosCluster
)
