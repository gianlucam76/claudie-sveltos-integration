# claudie-sveltos-integration

[Claudie](https://github.com/berops/claudie) simplifies the process of programmatically establishing Kubernetes clusters from a management cluster across multiple cloud vendors and on-prem datacenters.

[Sveltos](https://github.com/projectsveltos) streamlines the management of add-ons across a fleet of #Kubernetes clusters from a management cluster.

By integrating the two, we are able to manage Kubernetes clusters and add-ons in a unified way.

Goal of this repo is to contain a controller that watches for Claudie's Secrets with managed Kubernetes cluster Kubeconfig and creates corresponding SveltosCluster.
