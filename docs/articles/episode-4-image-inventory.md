# You Have No Idea What Docker Images Are Actually Running in Your Cluster

*This is part 4 of 7 of the* SRE Portal in Practice *series. [Episode 0 — Introduction](https://medium.com/@david_sabatie/sre-portal-a-kubernetes-native-dns-discovery-dashboard-for-platform-teams-009637868e00)*

---

## The Call You Don't Want on a Friday Afternoon

Security sends a message at 4:47 PM: "We have a CVE affecting all images from `registry.acme.internal`. Which of our services are using it?"

You open your terminal. You have four clusters, a dozen namespaces, and no central list of what's running where. You start pulling `kubectl get deployments -A -o json` across clusters, piping it through `jq`, pasting results into a spreadsheet, and realizing halfway through that you forgot StatefulSets, DaemonSets, and the CronJobs in the batch namespace.

An hour later you have something that might be right. Might be.

This is one of the most common blind spots on a platform team: there is no authoritative, always-fresh inventory of which container images are running in your clusters. You have observability for metrics and logs, but image provenance is scattered across Git, deployment pipelines, and the live cluster state — and those three things are rarely in sync.

The situation gets worse when you add image mutation into the picture. An admission webhook rewrites `nginx:1.25` to `registry.acme.internal/nginx:1.25-approved`. What the workload declares in its spec and what the pod actually runs can be two different things. Which one does security care about? Both. Can you easily show both? Almost certainly not.

---

## The Concept: ImageInventory and ImageRegistry

SRE Portal introduces two CRDs to solve this: `ImageInventory` and `ImageRegistry`.

An `ImageInventory` is a scanning configuration. You create one, point it at a Portal, and the controller periodically walks your cluster workloads — Deployments, StatefulSets, DaemonSets, CronJobs, Jobs — and collects every container image it finds. It looks at both the workload spec (the declared image) and the running pod (the actual image), and records both. If they differ, it marks the entry with a `changeType`: `mutated` when a webhook rewrote the tag, `injected` when a sidecar was added that wasn't in the original spec.

The controller doesn't dump everything into a single resource. Instead, it fans out into `ImageRegistry` child CRs — one per (registry host, namespace) pair. So if your `production` namespace runs images from `ghcr.io`, `docker.io`, and `registry.acme.internal`, you get three `ImageRegistry` objects. Each one carries the full image list for that registry in that namespace, including workload references (which Deployment, which container).

`ImageRegistry` CRs are controller-managed. You don't create or edit them — that's the operator's job. What you get from them is a queryable, structured inventory that's always at most one scan interval stale.

The `ImageInventory` controller also goes beyond raw enumeration. For each image tagged with a semantic version, it queries the upstream registry and checks whether a newer version exists. The result lands in `status.upgradeAvailableCount` on each `ImageRegistry`. You can see at a glance not just what's running, but what's outdated.

---

## In Practice

Here is a minimal `ImageInventory` scoped to a single namespace:

```yaml
apiVersion: sreportal.io/v1alpha1
kind: ImageInventory
metadata:
  name: production-inventory
  namespace: sreportal-system  # ImageInventory lives in the operator namespace
spec:
  portalRef: prod-cluster        # must match an existing Portal CR name
  namespaceFilter: production    # scan only this namespace; omit for all namespaces
  interval: 10m                  # refresh every 10 minutes; default is 5m
```

That's the minimum. The controller fills in defaults for everything else: it scans all five workload kinds and applies no label filtering.

To restrict which workload types are scanned, use `watchedKinds`:

```yaml
spec:
  portalRef: prod-cluster
  namespaceFilter: production
  watchedKinds:
    - Deployment
    - StatefulSet
  # CronJob, DaemonSet, Job are excluded
```

To focus on a specific team's workloads using their labels:

```yaml
spec:
  portalRef: prod-cluster
  namespaceFilter: production
  labelSelector: "team=payments,tier=backend"
```

After applying the `ImageInventory`, the controller runs its first scan and creates child `ImageRegistry` CRs automatically:

```bash
$ kubectl get imageregistries -n sreportal-system

NAME           HOST                       PORTAL        NAMESPACE    IMAGES   UPGRADES
a3f1b2c4d5e6   docker.io                  prod-cluster  production   14       3
b7e9f0a1c2d3   ghcr.io                    prod-cluster  production   7        1
c4a8b1d2e3f5   registry.acme.internal     prod-cluster  production   22       0
```

Each CR carries the full image list in its `spec.images`, with per-image fields:

- `originalImage` — what the workload's PodSpec declares
- `mutatedImage` — what the running pod actually uses
- `changeType` — `none`, `mutated`, or `injected`
- `repository`, `originalTag`, `tagType` — parsed for registry lookups
- `workloads` — list of (kind, namespace, name, container) referencing this image

Status per image entry includes `latestVersion` (highest semver tag found upstream) and `upgradeAvailable`.

**Answering the Friday security question** now looks like this:

```bash
kubectl get imageregistries -n sreportal-system \
  -l sreportal.io/portal=prod-cluster \
  -o json | jq '
    .items[]
    | select(.spec.host == "registry.acme.internal")
    | .spec.images[].workloads[]
    | "\(.namespace)/\(.name) (\(.kind))"
  '
```

Exact list. Seconds.

### Multi-Cluster: the `isRemote` Flag

When you run multiple clusters, each with its own SRE Portal instance, you don't want to duplicate scanning logic. Set `isRemote: true` on an `ImageInventory` in your central portal, and the controller fetches the image data from the remote portal's Connect API instead of scanning the local cluster:

```yaml
apiVersion: sreportal.io/v1alpha1
kind: ImageInventory
metadata:
  name: staging-shadow
  namespace: sreportal-system
spec:
  portalRef: staging-cluster  # a Portal that points to the remote cluster's API
  isRemote: true
```

The central portal aggregates `ImageRegistry` CRs from all federated clusters. The web UI shows the breakdown per portal.

---

## Why It Matters

**Incident response is faster.** The security question above takes seconds, not an hour. The data is always there, already aggregated.

**Mutation is visible.** Admission webhooks are common — image tag policies, registry mirrors, sidecar injectors. SRE Portal makes the gap between declared and actual explicit. You can see at a glance which workloads are running something different from what's in Git.

**Upgrade debt is quantified.** The `upgradeAvailableCount` field on each `ImageRegistry` gives you a concrete number. You can write a ServiceMonitor scraping the SRE Portal metrics endpoint and alert when the count crosses a threshold. No more manually running `trivy` scans or grepping Dependabot PRs.

**Scope is controlled.** `namespaceFilter` and `labelSelector` mean you can give each team a scoped `ImageInventory` that covers only their workloads. Security can have a cluster-wide one. The operator handles both without stepping on each other.

**It's declarative and self-healing.** If a workload is deleted, the image disappears from the next scan. If a new Deployment is rolled out, it appears within one interval. No pipeline step, no annotation required — the controller observes the cluster state continuously.

---

## What's Next

You now have a live, structured map of every image running in your cluster — where it came from, what it's actually running (not just what was declared), and whether it's out of date.

Next, we go one layer deeper: who talks to whom. Episode 5 covers network topology discovery from your existing `NetworkPolicy` resources — automatically building a graph of allowed traffic flows across namespaces, without any instrumentation.

Repository: [github.com/golgoth31/sreportal](https://github.com/golgoth31/sreportal)

---

*Built together with [Benjamin Colmart](https://github.com/benjamincolmart)*
