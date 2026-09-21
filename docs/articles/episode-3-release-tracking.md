# You Can't Tell What Was Released and When — And Neither Can Your On-Call

*This is part 3 of 7 of the* ***SRE Portal in Practice*** *series. [Episode 0 — Introduction](https://medium.com/@david_sabatie/sre-portal-a-kubernetes-native-dns-discovery-dashboard-for-platform-teams-009637868e00)*

---

## The Call Nobody Wants to Get

It's 2:47 AM. Error rates are climbing on the checkout service. You've pulled the dashboards, checked the logs, and you're about to start digging into the code — when someone on the bridge asks the question that stops everything:

*"Was there a release tonight?"*

Silence. Then: "I think so? Someone might have pushed something around midnight." You check Slack. You check the CI dashboard. You check the deployment annotations in the cluster. Each tool gives you a partial answer. The CI pipeline ran. Did it deploy? Which version? Was there a rollback? Who triggered it?

Fifteen minutes later you've assembled the answer from three sources: yes, there was a deployment at 00:23, version 2.4.1, and it turns out there was a silent rollback at 01:05. That's the culprit. But you've wasted a quarter of your incident response time just answering a question that should have a 10-second answer.

---

## Why Release Context Is Always Somewhere Else

This problem isn't a tooling gap — it's a placement gap. Your CI/CD system knows exactly what deployed and when. Your incident response happens inside the cluster, via runbooks and dashboards tied to services. These two worlds rarely talk directly.

The instinct is to add release context to your observability platform. That works, until your observability platform is part of the incident. Or until the person on call is a junior engineer who hasn't memorized where to look. Or until the service in question has releases spread across three different pipelines.

What platform teams actually need is release history surfaced in the same place engineers look when something breaks — next to the service DNS, the active alerts, and the component status. Not in a separate tab. Not in another tool. Right there.

---

## The Concept: A Release CRD That Lives Next to Your Service

SRE Portal adds a `Release` Custom Resource that lives in the same namespace as your service and links directly to a Portal — the same Portal that surfaces DNS and alerts.

A `Release` CR holds a list of `ReleaseEntry` items. Each entry captures what happened: a deployment, a rollback, or a hotfix. Entries carry the version, who triggered it, when it happened, a short message, and an optional link to the CI run or pull request.

The model is intentionally **push-based**. There is no webhook, no polling, no integration to configure inside the operator. Your CI/CD pipeline creates or updates the `Release` CR with `kubectl apply` — the same way you'd apply any Kubernetes manifest. The operator stores it. The web UI serves it. That's the entire integration surface.

This simplicity is a deliberate choice. Push-based means your pipeline owns the data. The operator is just durable storage with a web interface. It works with any CI/CD system that can run `kubectl`.

---

## In Practice

### The Release CR

```yaml
apiVersion: sreportal.io/v1alpha1
kind: Release
metadata:
  name: checkout-releases          # One CR per service, or per day — your choice
  namespace: production
spec:
  portalRef:
    name: checkout-portal          # Links to the Portal CR for this service
  entries:
    - type: deployment             # deployment | rollback | hotfix
      version: "2.4.1"
      origin: "github-actions"     # Which pipeline or system pushed this
      author: "alice"
      date: "2024-01-15T00:23:00Z"
      message: "Release 2.4.1: faster cart calculations, updated dependencies"
      link: "https://github.com/org/checkout/actions/runs/12345678"

    - type: rollback               # Rollback entry — same CR, appended
      version: "2.3.9"
      origin: "github-actions"
      author: "on-call-bot"
      date: "2024-01-15T01:05:00Z"
      message: "Automated rollback: p99 latency exceeded threshold (820ms > 500ms)"
      link: "https://github.com/org/checkout/actions/runs/12345699"
```

Key points:
- `spec.portalRef.name` ties this CR to a specific Portal, the same way alerts and DNS are tied.
- `spec.entries` is a list — you can append entries to a single CR (e.g., all releases for a service) or create a new CR per release. Both patterns work.
- `type` drives the badge colour in the UI: deployment is green, rollback is orange, hotfix is red.
- The operator updates `status.entryCount` after each reconciliation — useful for quick audits.

### Pushing from GitHub Actions

This is what the push step looks like in a real pipeline. No custom action, no SDK — just `kubectl apply` with an inline patch.

```yaml
- name: Record release in SRE Portal
  env:
    RELEASE_VERSION: ${{ github.ref_name }}
    RELEASE_AUTHOR: ${{ github.actor }}
    RELEASE_LINK: ${{ github.server_url }}/${{ github.repository }}/actions/runs/${{ github.run_id }}
  run: |
    # Build a patch with the new entry and apply it to the existing CR.
    # The operator merges entries — existing ones are not overwritten.
    cat <<EOF | kubectl apply -f -
    apiVersion: sreportal.io/v1alpha1
    kind: Release
    metadata:
      name: checkout-releases
      namespace: production
    spec:
      portalRef:
        name: checkout-portal
      entries:
        - type: deployment
          version: "${RELEASE_VERSION}"
          origin: "github-actions"
          author: "${RELEASE_AUTHOR}"
          date: "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
          message: "Deployed ${RELEASE_VERSION} via CI pipeline"
          link: "${RELEASE_LINK}"
    EOF
```

> **Note:** In a real setup you'd accumulate entries by fetching the existing CR and appending. The snippet above is simplified for clarity. A helper script or a small shell function handles the read-modify-apply cycle in practice.

Your rollback pipeline does the same thing with `type: rollback`. The operator doesn't care who writes the entry — it stores whatever the pipeline pushes.

### What You See in the Web UI

The Portal page for `checkout-portal` gains a Release Timeline section alongside the existing DNS table and alerts list. Entries are sorted by date (newest first). Each entry shows:

- A coloured type badge (deployment / rollback / hotfix)
- Version and author
- A relative timestamp ("2 hours ago") with the ISO date on hover
- The message
- A link to the CI run or PR if one was provided

When you're on a 3 AM bridge call and someone asks "was there a release tonight?", you open the portal page and the answer is at the top of the timeline. Ten seconds, not fifteen minutes.

---

## Why It Matters

**Faster incident triage.** The most common first question during an incident — "what changed recently?" — now has a direct answer in the same UI where you're looking at alerts and DNS. No context-switching.

**Rollback visibility.** Automated rollbacks are easy to miss. A rollback entry in the timeline is a first-class event with its own badge and message. Engineers on the next shift can see exactly what the on-call bot did and why.

**No new integration surface.** Any team that can run `kubectl apply` can push release data. There's nothing to configure inside the operator, no API key to manage, no webhook to register. The CI/CD system stays the source of truth — the portal just makes it visible where it's needed.

**Works across pipelines.** Services deployed by different CI systems (GitHub Actions, GitLab CI, ArgoCD, a custom script) all push to the same Release CR format. The portal aggregates them into a single timeline.

---

## What's Next

Knowing what *changed* is half the battle. The other half is knowing *what's actually running* — the exact Docker image, its digest, and when it was last pulled. In Episode 4, we look at image tracking: surfacing container image versions and registry metadata directly in the portal, so you can answer "is this cluster running what we think it's running?" without SSHing into nodes or parsing pod specs by hand.

The code is at [github.com/golgoth31/sreportal](https://github.com/golgoth31/sreportal). Issues and PRs welcome.

---

*Built together with [Benjamin Colmart](https://github.com/benjamincolmart)*
