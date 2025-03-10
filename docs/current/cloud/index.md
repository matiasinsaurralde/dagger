---
slug: /cloud
---

# Dagger Cloud

Dagger Cloud provides pipeline visualization, operational insights, and distributed caching for your Daggerized pipelines. The Dagger Engine and Dagger Cloud form the Dagger Platform, with Dagger Cloud providing a production-grade control plane.

[Learn more about Dagger Cloud](https://dagger.io/cloud)

Ready to connect your Dagger Engines? [Get started with Dagger Cloud](./572923-get-started.md).

## Features

### Pipeline visualization

Dagger Cloud provides a web interface to visualize each step of your pipeline, drill down to detailed logs, understand how long operations took to run, and whether operations were cached.

### Operational insights

Dagger Cloud collects telemetry from all your organization's Dagger Engines, whether they run in development or CI, and presents it all to you in one place. This gives you a unique view on all pipelines, both pre-push and post-push.

### Distributed caching

One of Dagger's superpowers is that it caches everything. On a single machine (like a laptop or long-running server) caching just works, because the same Dagger Engine writing to the cache is also reading from it. But in a multi-machine configuration (like an elastic CI cluster), things get more complicated because all machines are continuously producing and consuming large amounts of cache data. How do we get the right cache data to the right machine at the right time, without wasting compute, networking, or storage resources?

This is a complex problem which requires a distributed caching service, to orchestrate the movement of data between all machines in the cluster, and a centralized storage service. Because Dagger Cloud receives telemetry from all Dagger Engines, it can model the state of the cluster and make optimal caching decisions. The more telemetry data it receives, the smarter it becomes.

## Supported compute and CI platforms

Dagger Cloud is a "bring your own compute" service. The Dagger Engine can run on a wide variety of machines, including most development and CI platforms. If the Dagger Engine can run on it, then Dagger Cloud supports it.

Whether you're using Github Actions, GitLab, CircleCI, Jenkins or Tekton, Dagger Cloud connects to your Dagger Engines wherever they are deployed.
