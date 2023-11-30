# claudie-sveltos-integration

|  |  |
|---|---|
| <img src="https://raw.githubusercontent.com/berops/claudie/17480b6cb809fe795d454588af18355c7543f37e/docs/logo%20claudie_blue_no_BG.svg" width="250" alt="Claudie logo"> | [Claudie](https://github.com/berops/claudie) simplifies the process of programmatically establishing Kubernetes clusters from a management cluster across multiple cloud vendors and on-prem datacenters. |
| <img src="https://raw.githubusercontent.com/projectsveltos/sveltos/main/docs/assets/logo.png" width="250" alt="Sveltos logo"> | [Sveltos](https://github.com/projectsveltos) is a Kubernetes add-on controller that simplifies the deployment and management of add-ons and applications across multiple clusters. It runs in the management cluster and can programmatically deploy and manage add-ons and applications on any cluster in the fleet, including the management cluster itself. Sveltos supports a variety of add-on formats, including Helm charts, raw YAML, Kustomize, Carvel ytt, and Jsonnet. |


By integrating the two, we are able to manage Kubernetes clusters and add-ons in a unified way.

Goal of this repo is to contain a controller that watches for Claudie's Secrets with managed Kubernetes cluster Kubeconfig and creates corresponding SveltosCluster.
