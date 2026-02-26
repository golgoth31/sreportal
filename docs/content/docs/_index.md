---
title: Documentation
breadcrumbs: false
---

SRE Portal is a Kubernetes operator that discovers DNS records from your cluster resources and presents them in a unified web dashboard. It integrates with external-dns sources (Services, Ingresses, Istio, DNSEndpoints) and supports manual DNS entries through Custom Resources.

## Explore

{{< cards >}}
  {{< card link="getting-started" title="Getting Started" subtitle="Install the operator and create your first portal." icon="play" >}}
  {{< card link="architecture" title="Architecture" subtitle="CRD relationships, controller patterns, and data flow." icon="cube" >}}
  {{< card link="configuration" title="Configuration" subtitle="Operator ConfigMap reference for sources, grouping, and timing." icon="adjustments" >}}
  {{< card link="annotations" title="Annotations" subtitle="Route endpoints to portals and assign groups with annotations." icon="tag" >}}
  {{< card link="web-ui" title="Web UI" subtitle="Dashboard routes, filters, and portal navigation." icon="desktop-computer" >}}
  {{< card link="mcp" title="MCP Server" subtitle="AI assistant integration with Claude Desktop, Claude Code, and Cursor." icon="chip" >}}
  {{< card link="api" title="API Reference" subtitle="Auto-generated CRD field reference." icon="code" >}}
  {{< card link="development" title="Development" subtitle="Build, test, lint, and contribute to the project." icon="terminal" >}}
{{< /cards >}}
