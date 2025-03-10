---
slug: /194031/kubernetes
displayed_sidebar: "current"
category: "guides"
tags: ["kubernetes"]
authors: ["Gerhard Lazu", "Vikram Vaswani"]
date: "2023-11-30"
---

# Run Dagger on Kubernetes

## Introduction

This guide outlines how to run, and connect to, the Dagger Engine on Kubernetes.

## Assumptions

This guide assumes that you have:

- The [Dagger CLI](../cli/465058-install.md) installed locally.
- [Helm](https://helm.sh) v3.x available locally.
- A running Kubernetes cluster (tested with Kubernetes v1.28).

## Step 1: Deploy Dagger with Helm

Deploy Dagger on your Kubernetes cluster with Helm:

```shell
helm upgrade --install --namespace=dagger --create-namespace \
    dagger oci://registry.dagger.io/dagger-helm
```

Wait for the Dagger Engine to become ready:

```shell
kubectl wait --for condition=Ready --timeout=60s pod \
    --selector=name=dagger-dagger-helm-engine --namespace=dagger
```

You can find more information on what was deployed using the following command:

```shell
kubectl describe daemonset/dagger-dagger-helm-engine --namespace=dagger
```

## Step 2: Connect Dagger CLI to Dagger Engine pod

Get a Dagger Engine pod name:

```shell
DAGGER_ENGINE_POD_NAME="$(kubectl get pod \
    --selector=name=dagger-dagger-helm-engine --namespace=dagger \
    --output=jsonpath='{.items[0].metadata.name}')"
export DAGGER_ENGINE_POD_NAME
```

Next, set the `_EXPERIMENTAL_DAGGER_RUNNER_HOST` variable so that the Dagger CLI knows to connect to the Dagger Engine that you deployed as a Kubernetes pod:

```shell
_EXPERIMENTAL_DAGGER_RUNNER_HOST="kube-pod://$DAGGER_ENGINE_POD_NAME?namespace=dagger"
export _EXPERIMENTAL_DAGGER_RUNNER_HOST
```

Finally, run an operation that shows the kernel info of the Kubernetes node where this Dagger Engine runs:

```shell
dagger query <<EOF
{
    container {
        from(address:"alpine") {
            withExec(args: ["uname", "-a"]) { stdout }
        }
    }
}
EOF
```

This is what a successful response should look like:

```shell
┣─╮
│ ▽ init
│ █ [0.64s] connect
│ ┣ [0.52s] starting engine
│ ┣ [0.12s] starting session
│ ┃ OK!
│ ┻
█ [2.44s] dagger query
┣ [0.00s] loading module
█ [2.44s] query
┃ {
┃     "container": {
┃         "from": {
┃             "withExec": {
┃                 "stdout": "Linux buildkitsandbox 6.1.0-12-amd64 #1 SMP PREEMPT_DYNAMIC Debian 6.1.52-1 (2023-09-07) x86_64 Linux\n"
┃             }
┃         }
┃     }
┃ }
┣─╮
│ ▽ from alpine
│ █ [1.26s] resolve image config for docker.io/library/alpine:latest
│ █ [0.18s] pull docker.io/library/alpine:latest
│ ┣ [0.03s] resolve docker.io/library/alpine@sha256:34871e7290500828b39e22294660bee86d966bc0017544e848dd9a255cdf59e0
│ ┣ [0.61s] ███████████████████ sha256:c926b61bad3b94ae7351bafd0c184c159ebf0643b085f7ef1d47ecdc7316833c
│ ┣ [0.18s] extracting sha256:c926b61bad3b94ae7351bafd0c184c159ebf0643b085f7ef1d47ecdc7316833c
│ ┣─╮ pull docker.io/library/alpine:latest
│ ┻ │
█◀──╯ [0.24s] exec uname -a
┃     Linux buildkitsandbox 6.1.0-12-amd64 #1 SMP PREEMPT_DYNAMIC Debian 6.1.52-1 (2023-09-07) x86_64 Linux
┻
• Engine: dagger-dagger-helm-engine-bvbtk (version v0.9.3)
⧗ 3.10s ✔ 12
```

The line above starting with `Engine:` confirms the Dagger Engine that the CLI connected to.

To double-check that the operations are running on the Kubernetes cluster, follow the pod logs:

```shell
kubectl logs pod/$DAGGER_ENGINE_POD_NAME --namespace=dagger --follow
```

## Conclusion

This guide demonstrated the simplest approach to using Dagger on Kubernetes. For more complex scenarios, such as setting up a Continuous Integration (CI) environment with Dagger on Kubernetes, use the following resources:

- Understand [CI architecture patterns on Kubernetes with Dagger](./237420-ci-architecture-kubernetes.md)
- See an example of [running Dagger on Amazon EKS with GitHub Actions Runner and Karpenter](./934191-eks-github-karpenter.md)
- [Dagger Cloud](https://docs.dagger.io/cloud)
- [Dagger GraphQL API](https://docs.dagger.io/api/975146/concepts)
- Dagger [Go](https://docs.dagger.io/sdk/go), [Node.js](https://docs.dagger.io/sdk/nodejs) and [Python](https://docs.dagger.io/sdk/python) SDKs

:::info
If you need help troubleshooting your Dagger deployment on Kubernetes, let us know in [Discord](https://discord.com/invite/dagger-io) or create a [GitHub issue](https://github.com/dagger/dagger/issues/new/choose).
:::
