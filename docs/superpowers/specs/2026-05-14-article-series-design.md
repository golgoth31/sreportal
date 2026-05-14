# SRE Portal — Article Series Design Spec

**Date:** 2026-05-14
**Authors:** David Sabatie, Benjamin Colmart ([@benjamincolmart](https://github.com/benjamincolmart))
**Status:** Approved

---

## Context

SRE Portal is a Kubernetes-native operator that provides DNS discovery, service grouping, alert correlation, release tracking, image inventory, network flow discovery, and status pages — all from a single binary with a web UI, a Connect gRPC API, and MCP servers.

The introductory article (Episode 0) is already published on Medium:
https://medium.com/@david_sabatie/sre-portal-a-kubernetes-native-dns-discovery-dashboard-for-platform-teams-009637868e00

This spec defines the content plan for Episodes 1–7, a "Problem → Solution" series that builds on the intro.

---

## Series Overview

**Title:** *SRE Portal in Practice*
**Format:** 7 articles, ~1000–1500 words each
**Language:** English
**Audience:** Platform engineers and SREs comfortable with Kubernetes, with accessibility for the broader DevOps audience (no deep operator internals assumed)
**Publication:** Medium (primary) + LinkedIn cross-post
**Co-developer credit:** Benjamin Colmart ([github.com/benjamincolmart](https://github.com/benjamincolmart)) — mentioned in the intro paragraph or author footer of every article

---

## Article Structure Template

Each article follows this structure:

1. **Hook** — The pain, told as a real on-call or day-to-day situation (2–3 paragraphs, no jargon)
2. **The Concept** — What the feature is and how it works, explained accessibly (CRD spec simplified, key fields only)
3. **In Practice** — Annotated YAML snippets + web UI screenshot description, step-by-step walkthrough
4. **Why It Matters** — Concrete team benefits: less toil, faster MTTR, better visibility
5. **What's Next** — One-sentence teaser for the next article + link to the repo

---

## Episodes

### Episode 1 — DNS Discovery
**Title:** *"You don't know what's actually exposed in your cluster"*
**Problem:** In a multi-cluster environment with dozens of services, there's no single place to see which FQDNs are active, who owns them, or which cluster they live in. Teams rely on tribal knowledge or Confluence pages that are always out of date.
**Feature:** DNS CRD, DNSRecord CRD, external-dns integration, portal routing via `sreportal.io/portal` annotation
**Key concepts:**
- The `DNS` CRD as a scoped entry point for DNS zone management
- `DNSRecord` CRD — individual records auto-synced to external-dns
- The `sreportal.io/portal` annotation on Services/Ingresses for automatic grouping
- The web UI Links page: unified FQDN dashboard
**In Practice YAML:** `DNS` CR + annotated `Service` + resulting `DNSRecord` + web UI walkthrough
**Teaser:** "Now you can see what's exposed — next, we'll make sense of the alerts that fire on those services."

---

### Episode 2 — Portal & Alertmanager
**Title:** *"Your alerts have no context — and that costs you on-call time"*
**Problem:** When PagerDuty wakes you up at 3am, you get a raw alert with no link to the service it belongs to, no FQDN, no team. You spend the first 10 minutes just figuring out *what* is affected.
**Feature:** Portal CRD, Alertmanager CRD, active alert fetching, per-portal Alerts page in the web UI
**Key concepts:**
- `Portal` CRD as a logical grouping of services (links DNS, Alertmanager, Releases, Images)
- `Alertmanager` CRD — local or remote URL, linked to a Portal via `portalRef`
- The controller periodically fetches active alerts and stores them in `.status`
- The web UI Alerts page: alerts filtered by portal
**In Practice YAML:** `Portal` CR + `Alertmanager` CR + web UI Alerts walkthrough
**Teaser:** "Once you know what's firing and on which service, the next question is: what changed? Releases are next."

---

### Episode 3 — Release Tracking
**Title:** *"You can't tell what was released and when — and neither can your on-call"*
**Problem:** Something breaks after a deployment. You ask "was there a release today?" and nobody knows for sure without checking three different tools. Release history isn't surfaced where engineers need it — next to the service, inside the platform.
**Feature:** Release CRD, per-portal release timeline
**Key concepts:**
- `Release` CRD linked to a Portal via `portalRef`
- `ReleaseEntry` fields: type (deployment/rollback/hotfix), version, origin (CI/CD system), author, date, message, link
- Push-based model: CI/CD pipelines create `Release` CRs via `kubectl apply`
- Web UI: release timeline visible per portal alongside alerts and DNS
**In Practice YAML:** `Release` CR with multiple `entries` + GitHub Actions snippet pushing a release entry
**Teaser:** "Knowing what changed is half the battle. Next: knowing *what runs* — the Docker images in your cluster."

---

### Episode 4 — Docker Image Inventory
**Title:** *"You have no idea what Docker images are actually running in your cluster"*
**Problem:** Security asks: "Are you running any images from that compromised registry?" You check three clusters, six namespaces, and still aren't sure. There's no single inventory of what's running, where, and from which registry.
**Feature:** ImageInventory CRD, ImageRegistry CRD, periodic image scanning, remote portal federation
**Key concepts:**
- `ImageInventory` CRD linked to a Portal — scans Deployments, StatefulSets, DaemonSets, CronJobs, Jobs
- `namespaceFilter` and `labelSelector` for scoped scanning
- `interval` for configurable refresh (default 5m)
- `ImageRegistry` child CRs (auto-created): one per registry host × namespace
- `isRemote` flag: federate image inventory from a remote portal via Connect API
- Web UI: per-portal image inventory with registry breakdown
**In Practice YAML:** `ImageInventory` CR + resulting `ImageRegistry` CRs + MCP query example
**Teaser:** "You know what images run. Next: who talks to whom — network topology from your NetworkPolicies."

---

### Episode 5 — Network Flow Discovery
**Title:** *"Your network topology is a black box — until it isn't"*
**Problem:** A new engineer joins and asks: "Which services call the payments API?" You know roughly, but explaining the actual policy-enforced flows requires reading dozens of NetworkPolicy YAMLs. There's no graph, no visual, just YAML.
**Feature:** NetworkFlowDiscovery CRD, FlowObserver, FlowEdgeSet, FlowNodeSet, traffic observation
**Key concepts:**
- `NetworkFlowDiscovery` CRD — discovers flows from Kubernetes NetworkPolicies and FQDNNetworkPolicies, scoped by namespace list
- Nodes: service, cron, database, messaging, external endpoint types
- Edges: internal, cross-ns, cron, database, messaging, external flow types
- `FlowObserver` — enriches edges with live traffic observation (`used` / `evaluated` fields)
- `isRemote` flag: federate from remote portal
- Web UI: network graph visualization per portal
**In Practice YAML:** `NetworkFlowDiscovery` CR + description of resulting node/edge graph in the UI
**Teaser:** "You understand the topology. Next: how do you communicate outages to your users?"

---

### Episode 6 — Status Pages
**Title:** *"Your users are pinging you on Slack to ask if X is down"*
**Problem:** When an incident happens, 20 people flood your Slack DM asking "is the API down?" You're already fighting the fire and now managing comms manually. You need a proper status page — but standing one up feels like another project.
**Feature:** Component CRD, Incident CRD, Maintenance CRD, status page web UI (alpha)
**Key concepts:**
- `Component` CRD — a service component with health status, linked to a Portal
- `Incident` CRD — an active incident affecting one or more components, with severity and timeline
- `Maintenance` CRD — a scheduled maintenance window affecting components
- Status page rendered in the web UI: current status, active incidents, upcoming maintenance
- Alpha label: what's stable, what's still evolving
**In Practice YAML:** `Component` + `Incident` CR during a real incident scenario
**Teaser:** "Status pages: done. Last piece: let your AI assistant query all of this — that's what MCP servers are for."

---

### Episode 7 — MCP Servers
**Title:** *"Your AI assistant knows nothing about your infrastructure — SRE Portal changes that"*
**Problem:** You use Claude or Cursor daily. But when you ask "what services are exposed in the production cluster?" or "are there active alerts for the payments portal?", it has no idea. Your platform knowledge lives in Kubernetes, not in your AI context.
**Feature:** All MCP server endpoints — DNS, alerts, metrics, releases, netpol, status, image
**Key concepts:**
- Model Context Protocol: how Claude/Cursor connect to tools
- SRE Portal exposes MCP at `/mcp`, `/mcp/dns`, `/mcp/alerts`, `/mcp/metrics`, `/mcp/releases`, `/mcp/netpol`, `/mcp/status`, `/mcp/image`
- Each endpoint = a set of MCP tools the AI can call
- Demo: natural language queries → MCP tool calls → real cluster data
- Configuration: connecting Claude Code or Cursor to the MCP endpoint
**In Practice:** Claude Code config snippet + sample conversation showing infra queries answered from live cluster data
**Teaser:** Link to the repo + invitation to contribute / raise issues

---

## Cross-Cutting Conventions

- **Co-developer credit:** Every article includes "Built together with [Benjamin Colmart](https://github.com/benjamincolmart)" in the introduction or author bio section.
- **Repo link:** Every article links to the SRE Portal GitHub repository.
- **Series navigation:** Every article starts with "This is part N of the *SRE Portal in Practice* series" and links the full series index (Medium publication or pinned list).
- **YAML snippets:** Always complete, copyable, with comments on key fields. No placeholder values without a note.
- **Screenshots/UI:** Described in text if no screenshot available; author adds real screenshots before publishing.
- **Alpha label:** Status Pages (Episode 6) explicitly flags alpha status and invites feedback.

---

## Publishing Checklist (per article)

- [ ] Draft written in Markdown
- [ ] YAML snippets tested against a local cluster
- [ ] Co-developer credit present
- [ ] Series navigation links updated
- [ ] Medium draft created and formatted
- [ ] LinkedIn post drafted (shorter version, links to Medium)
- [ ] Repo link included
