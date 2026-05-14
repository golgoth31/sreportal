# Your AI Assistant Knows Nothing About Your Infrastructure — SRE Portal Changes That

*This is part 7 of 7 of the* ***SRE Portal in Practice*** *series. [Episode 0 — Introduction](https://medium.com/@david_sabatie/sre-portal-a-kubernetes-native-dns-discovery-dashboard-for-platform-teams-009637868e00)*

---

## The Context Gap

You're deep in an incident. Error rates are climbing on the payments service. You open Claude Code — it's already in your terminal, and honestly it's been useful all week for reading logs and writing runbooks. So you ask it the natural question:

*"What services are currently exposed in the production cluster for the payments portal?"*

Silence. Then a polite admission that it doesn't have access to your cluster.

You try again: *"Are there any active alerts for payments right now?"*

Same answer. Of course — your AI assistant only knows what you've pasted into the conversation. It has no idea what's running. It can't see your Kubernetes state, your Alertmanager feeds, your deployment history, or your DNS records. It's a very capable analyst working with a blindfold on.

This is the context gap. Your platform knowledge lives in the cluster. Your AI assistant lives outside it. Bridging that gap used to mean writing custom integrations, maintaining scripts, or spending the first five minutes of every AI session pasting in raw `kubectl` output.

SRE Portal closes the gap with MCP.

---

## What MCP Is (Briefly)

Model Context Protocol is an open standard for connecting AI assistants to external tools and data sources. Think of it as a well-defined plugin interface: an MCP server exposes a set of tools the AI can call at runtime, with typed inputs and outputs. The AI decides when to call them based on your conversation.

It's now supported by Claude Code, Cursor, and a growing list of other AI tools. Once you configure an MCP server, the AI can query it mid-conversation — transparently, without you having to manually fetch and paste data.

SRE Portal implements the Streamable HTTP transport using the `mark3labs/mcp-go` library. All MCP endpoints are served from the same single binary that runs the operator, the web UI, and the gRPC API. No sidecar, no separate deployment.

---

## What SRE Portal Exposes

SRE Portal ships with multiple MCP endpoints, each scoped to a domain:

| Endpoint | What it covers |
|---|---|
| `/mcp` and `/mcp/dns` | DNS discovery: FQDNs, sync status, portal routing |
| `/mcp/alerts` | Active Alertmanager alerts per portal |
| `/mcp/metrics` | Prometheus metrics from the operator |
| `/mcp/releases` | Deployment history per portal |
| `/mcp/netpol` | Network flow topology between services |
| `/mcp/status` | Status page data: components, incidents, maintenance |
| `/mcp/image` | Docker image inventory across portals |

Each endpoint exposes a set of tools the AI can call. For example, `/mcp/dns` exposes `list_fqdns` and `get_portal`, `/mcp/alerts` exposes `get_alerts`, and `/mcp/image` exposes `get_images`. The AI picks the right tool based on what you ask.

---

## Configuration

Adding SRE Portal as an MCP server in Claude Code takes one entry in `~/.claude/settings.json`:

```json
{
  "mcpServers": {
    "sreportal": {
      "type": "http",
      "url": "https://sreportal.your-cluster.example.com/mcp"
    },
    "sreportal-alerts": {
      "type": "http",
      "url": "https://sreportal.your-cluster.example.com/mcp/alerts"
    },
    "sreportal-releases": {
      "type": "http",
      "url": "https://sreportal.your-cluster.example.com/mcp/releases"
    },
    "sreportal-images": {
      "type": "http",
      "url": "https://sreportal.your-cluster.example.com/mcp/image"
    }
  }
}
```

For Cursor, the same URLs go into the MCP section of your workspace settings. The transport is Streamable HTTP — no SSE server, no subprocess to manage, no stdio pipe.

That's the full setup. Restart Claude Code, and the tools are available.

---

## In Practice

Here's what a real investigation looks like once SRE Portal is connected. An on-call engineer opens Claude Code during an incident:

---

**Engineer:** What FQDNs are currently live for the payments portal?

**Claude:** *(calls `list_fqdns` on `/mcp/dns` with portal filter `payments`)*

The payments portal has 4 active FQDNs:
- `api.payments.prod.example.com` → 203.0.113.42 (A, synced)
- `checkout.payments.prod.example.com` → 203.0.113.42 (A, synced)
- `webhooks.payments.prod.example.com` → 203.0.113.55 (A, synced)
- `legacy-api.payments.prod.example.com` → CNAME to api.payments.prod.example.com (synced)

---

**Engineer:** Any active alerts right now?

**Claude:** *(calls `get_alerts` on `/mcp/alerts` with portal `payments`)*

Yes, 2 active alerts:
- `HighErrorRate` — firing since 14:32 UTC, labels: service=checkout, severity=critical
- `SlowDatabaseQueries` — firing since 14:28 UTC, labels: service=payments-api, severity=warning

The `HighErrorRate` alert fired about 4 minutes after `SlowDatabaseQueries`. That sequencing suggests the database issue may be upstream.

---

**Engineer:** Was there a recent deployment to payments?

**Claude:** *(calls `get_releases` on `/mcp/releases` with portal `payments`)*

Last 3 releases for payments portal:
- `payments-api` v2.7.1 — deployed 13:55 UTC (37 minutes ago)
- `checkout` v1.4.0 — deployed 11:20 UTC
- `payments-api` v2.7.0 — deployed yesterday 09:14 UTC

The `payments-api` deployment at 13:55 is 33 minutes before the first alert. That's worth investigating.

---

This is not a scripted demo. Those are real MCP tool calls against live cluster data, happening mid-conversation. The AI isn't hallucinating — it's reading from the same source of truth your web dashboard uses.

---

## Why It Matters

The value here is not novelty. It's operational speed and reduced cognitive load.

**No context-switching.** During an incident you stay in one tool. You don't open the portal, open Alertmanager, open your CI dashboard, and manually correlate. You ask questions and get answers backed by live data.

**Junior engineers get context.** An engineer who's never seen the payments portal before can ask "what does this portal expose?" and get a factual, structured answer in seconds. That knowledge no longer lives only in the heads of the three people who've been on that team for two years.

**AI reasoning over real state.** When the AI has access to your DNS records, alert history, and deployment timeline simultaneously, it can spot correlations you might miss — like the deployment-to-alert timing above. It's not guessing; it's reasoning over actual data.

**It's already there.** If you're running SRE Portal, the MCP endpoints are live. No additional deployment, no new service to operate. You just add the URLs to your config.

---

## Wrapping the Series

This is the final episode of *SRE Portal in Practice*. Over seven episodes, we've covered the full scope of what the project does:

1. **DNS discovery** — a live, queryable inventory of every FQDN in your cluster
2. **Portals and Alertmanager** — grouping resources by team and surfacing active alerts
3. **Release tracking** — deployment history where you actually need it, during an incident
4. **Image inventory** — knowing exactly what's running and where
5. **Network topology** — understanding service-to-service flows without reading NetworkPolicy YAML
6. **Status pages** — communicating incidents and maintenance to stakeholders
7. **MCP servers** — putting all of that in front of your AI assistant

The thread through all of it is the same: platform knowledge that currently lives scattered across multiple tools, tribal memory, and stale documentation, pulled into a single operator and made available to both humans and AI.

If any of this resonates, the code is at [github.com/golgoth31/sreportal](https://github.com/golgoth31/sreportal). A star helps more people find it. Open an issue if something doesn't work or you have a feature that would close a gap in your own setup. Contributions are welcome — the architecture is designed to be extended, and the issues list has good first issues labeled.

Thanks for reading through the whole series.

---

*Built together with [Benjamin Colmart](https://github.com/benjamincolmart)*
