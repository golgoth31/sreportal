# Your Users Are Pinging You on Slack to Ask If X Is Down

*This is part 6 of 7 of the* ***SRE Portal in Practice*** *series.*
*[Episode 0 — Introduction](https://medium.com/@david_sabatie/sre-portal-a-kubernetes-native-dns-discovery-dashboard-for-platform-teams-009637868e00)*

---

## The Slack Thread That Shouldn't Exist

It's 14:07 on a Tuesday. Your payments API is returning 503s. You're three minutes into the incident: tailing logs, checking dashboards, assembling the right people on a call.

Then it starts. A Slack DM from a product manager: *"Hey, is the payments API down?"* Then another from a frontend engineer. Then someone from customer support forwarding a tweet. By the time you have a hypothesis about the root cause, you've also received the same question twelve times, each one pulling a fraction of your attention away from fixing the actual problem.

This is the hidden cost of incidents: **communication overhead**. Fixing the system is hard enough. Running a parallel public relations operation in your head — deciding who to tell, what to say, when to update — turns a focused engineering problem into an improvised multi-channel broadcast. You're the engineer and the spokesperson and the support tier, all at once.

The standard answer is: stand up a status page. Tools like Statuspage.io or Instatus are purpose-built for this. But if your infrastructure is already living in Kubernetes, managed as code, owned through GitOps — adding a separate SaaS product with its own auth, its own API, its own update mechanism feels like backsliding. You're back to the two-system problem: your real state lives in Kubernetes, but your communicated state lives somewhere else.

SRE Portal's status page feature is the Kubernetes-native answer to that gap.

---

## The Concept

Status pages in SRE Portal are built on three CRDs: `Component`, `Incident`, and `Maintenance`. They integrate with the same `Portal` object introduced in Episode 2, and they appear in the web UI alongside DNS discovery and alerts.

**`Component`** represents a service or capability your users care about. A component has a name, a description, and a `status` field with four possible values: `operational`, `degraded`, `outage`, or `maintenance`. It attaches to a Portal via `portalRef`. When nothing is wrong, you define your components once and leave them alone.

**`Incident`** represents an active problem. It carries a `severity` (`critical`, `major`, `minor`), a `title`, a `message`, and a list of `affectedComponents`. The controller tracks start and resolution timestamps. When an incident exists, the portal status page reflects it — component badges update, the incident timeline appears, and the overall portal health rolls up accordingly.

**`Maintenance`** is the scheduled counterpart. It has `scheduledStart`, `scheduledEnd`, `affectedComponents`, and the same title/message fields. It shows up in the UI as an upcoming or in-progress window, so users see it before you're forced to explain an unexpected outage.

Because these are Kubernetes resources, they fit naturally into GitOps. An incident is a `kubectl apply`. A resolution is a `kubectl patch` or a PR merge. You can automate incident creation from your alerting system, tie maintenance windows to your change management process, or just manage everything by hand through your normal cluster workflow — the mechanism is the same one you already use everywhere else.

---

## In Practice

### Step 1 — Enable the status page feature on your Portal

If you followed the earlier episodes, you already have a `Portal` resource. Add the `status` feature flag:

```yaml
apiVersion: sreportal.io/v1alpha1
kind: Portal
metadata:
  name: payments
  namespace: sreportal-system
spec:
  title: "Payments Platform"
  subPath: payments
  features:
    dns: true
    alerts: true
    status: true   # enables the status page for this portal
```

### Step 2 — Define your Components

Create a `Component` for each capability you want to communicate status on. Start with the ones users ask about most.

```yaml
apiVersion: sreportal.io/v1alpha1
kind: Component
metadata:
  name: payments-api
  namespace: sreportal-system
spec:
  portalRef:
    name: payments           # links this component to the payments portal
  displayName: "Payments API"
  description: "Core payment processing and authorization"
  status: operational        # operational | degraded | outage | maintenance
```

```yaml
apiVersion: sreportal.io/v1alpha1
kind: Component
metadata:
  name: payments-webhook
  namespace: sreportal-system
spec:
  portalRef:
    name: payments
  displayName: "Payment Webhooks"
  description: "Outbound webhook delivery to merchant systems"
  status: operational
```

At this point your status page shows two green components. No incidents, no noise.

### Step 3 — Declare an Incident

It's 14:07 again. The payments API is returning 503s. Instead of manually responding to every Slack message, you apply this:

```yaml
apiVersion: sreportal.io/v1alpha1
kind: Incident
metadata:
  name: payments-api-503-20260514
  namespace: sreportal-system
spec:
  portalRef:
    name: payments
  title: "Elevated error rate on Payments API"
  message: >
    We are investigating elevated 503 error rates on the Payments API.
    Payment processing may be intermittently failing. We will update
    this page as we learn more.
  severity: critical           # critical | major | minor
  affectedComponents:
    - payments-api             # reference to the Component name
  startedAt: "2026-05-14T14:07:00Z"
  # resolvedAt is set when you patch or delete the incident
```

The status page now shows:
- `payments-api` badge flips to **outage**
- The incident appears in the timeline with your message and severity
- The portal's overall health indicator reflects the active critical incident

When the incident is resolved:

```yaml
# patch the incident to add a resolution timestamp and message
spec:
  resolvedAt: "2026-05-14T14:51:00Z"
  message: >
    The root cause was a misconfigured connection pool limit after a
    recent deploy. The issue has been resolved and error rates are
    back to baseline.
```

That's the entire communication workflow. Everyone watching the status page sees the update the moment you apply it.

### Step 4 — Schedule a Maintenance Window

Next Tuesday you're upgrading the payments database. You know it will cause a brief outage for webhooks. You apply this a week in advance:

```yaml
apiVersion: sreportal.io/v1alpha1
kind: Maintenance
metadata:
  name: payments-db-upgrade-20260521
  namespace: sreportal-system
spec:
  portalRef:
    name: payments
  title: "Payments database upgrade"
  message: >
    We will be upgrading the payments database to PostgreSQL 17.
    Payment webhook delivery will be paused during this window.
    Core payment processing will remain available.
  scheduledStart: "2026-05-21T02:00:00Z"
  scheduledEnd: "2026-05-21T04:00:00Z"
  affectedComponents:
    - payments-webhook
```

The status page shows an upcoming maintenance window immediately. No separate calendar invite, no "heads up" Slack post that gets buried — it's on the page, visible to anyone who checks.

---

## Why It Matters

The immediate value is obvious: fewer interruptions during incidents. Anyone in your company can check the status page instead of DMing you.

The deeper value is that **your status data is now managed the same way your infrastructure data is**. An incident is a Kubernetes resource. It has a creation timestamp, an owner, a history in git if you use GitOps. You can audit who declared the incident, when, and how long it took to resolve — without a separate tool, without a separate login, without a manual export step.

Maintenance windows live in the same place as your deployments, your DNS records, your Alertmanager configs. Your runbooks can reference them. Your CI pipelines can create or close them. Your on-call automation can open an incident the moment an alert fires.

This is what "Kubernetes-native" actually means in practice: not just that something runs on Kubernetes, but that the operational artifacts — incidents, status, maintenance records — are cluster objects with the same lifecycle management you already apply to everything else.

---

## Alpha Status — Here's Where It Is

Status pages are in alpha in SRE Portal v1alpha1. The CRDs are implemented and the web UI renders component status, incident timelines, and maintenance windows. What's still evolving:

- More granular component status transitions (partial outage, investigating)
- History retention for resolved incidents
- Public-facing status page endpoints (unauthenticated access)
- Webhook notifications when incidents open or resolve

If you're using this in a real environment and running into edge cases, or if you have a clear picture of what your team needs from a Kubernetes-native status page — we want to hear from you. Open an issue on [github.com/golgoth31/sreportal](https://github.com/golgoth31/sreportal), or drop a comment here.

Alpha isn't a disclaimer. It's an invitation.

---

## What's Next

Status pages: done. You now have DNS discovery, alert context, release tracking, network policy visualization, image inventory, and a status page — all living as Kubernetes resources, all surfaced through a single portal.

The last piece is what ties all of this together for AI-assisted operations: **MCP servers**. SRE Portal exposes everything it knows — FQDNs, alerts, incidents, releases, network policies — as a Model Context Protocol server. Your AI assistant can query it directly. That's Episode 7.

Repository: [github.com/golgoth31/sreportal](https://github.com/golgoth31/sreportal)

---

*Built together with [Benjamin Colmart](https://github.com/benjamincolmart)*
