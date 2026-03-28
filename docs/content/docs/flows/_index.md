---
title: Controller Flows
weight: 3
---

Detailed step-by-step diagrams showing how data flows through each controller, from input to ReadStore projection and UI display.

{{< cards >}}
  {{< card link="dns-source" title="DNS Source Flow" subtitle="External-dns sources → DNSRecord CRs → ReadStore → UI. Full lifecycle with dedup and hash-based change detection." icon="globe-alt" >}}
  {{< card link="dns-controller" title="DNS Controller Flow" subtitle="Manual DNS entries + DNSRecord aggregation → DNS resolution → ReadStore projection." icon="document-text" >}}
  {{< card link="dnsrecord" title="DNSRecord Controller Flow" subtitle="Hash resync, DNS resolution, and FQDNView projection into the ReadStore." icon="database" >}}
  {{< card link="alertmanager" title="Alertmanager Flow" subtitle="Local and remote alert fetching → status update → ReadStore projection." icon="bell" >}}
  {{< card link="portal" title="Portal Flow" subtitle="Local status, remote sync (DNS, alerts, network flows), child CR lifecycle." icon="view-grid" >}}
  {{< card link="release" title="Release Flow" subtitle="Daily release tracking with TTL cleanup and ReadStore projection." icon="calendar" >}}
  {{< card link="network-flow-discovery" title="Network Flow Discovery Flow" subtitle="NetworkPolicy parsing, graph building, and FlowNodeSet/FlowEdgeSet management." icon="switch-horizontal" >}}
  {{< card link="component" title="Component Flow" subtitle="Platform component status computation with maintenance override via ReadStore." icon="check-circle" >}}
  {{< card link="maintenance" title="Maintenance Flow" subtitle="Phase lifecycle (upcoming → in_progress → completed) with strategic RequeueAfter." icon="clock" >}}
{{< /cards >}}
