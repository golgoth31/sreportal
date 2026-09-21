# You Don't Know What's Actually Exposed in Your Cluster

> This is part 1 of 7 of the *SRE Portal in Practice* series.
> [Episode 0 — Introduction](https://medium.com/@david_sabatie/sre-portal-a-kubernetes-native-dns-discovery-dashboard-for-platform-teams-009637868e00)

---

## The 2 AM Phone Call

It's late. An alert fires. A service is down and nobody on the call can agree on which hostname customers are actually hitting. Someone opens a browser tab — the Confluence page was last updated eight months ago. Someone else runs `kubectl get ingress -A` and pastes 200 lines of output into Slack. Three people cross-reference DNS records in Route 53. Another engineer is checking a different cluster entirely.

Twenty minutes of this before anyone agrees on the FQDN. The service itself was back up in four.

This is not an edge case. It's Tuesday.

The root problem is structural: Kubernetes doesn't have a native answer to "what FQDNs are live right now, where do they point, and who owns them?" external-dns writes records to your DNS provider — but it doesn't give you a searchable, cluster-wide view of what it wrote. Services and Ingresses accumulate over months. Teams come and go. The documentation drifts. What remains is tribal knowledge.

SRE Portal's DNS discovery feature exists to close that gap. It maintains a live, queryable inventory of every FQDN in your cluster — automatically, without a dedicated team to maintain it.

---

## The Concept

DNS discovery in SRE Portal is built on two CRDs and one annotation.

**`DNS`** is the scoped entry point. You create one per Portal (a logical grouping — usually one per team or product). It holds manual entries if you have them, and acts as the aggregation point for automatically discovered FQDNs in its status. Think of it as the "DNS namespace" for a portal.

**`DNSRecord`** is the automated discovery unit. You create one per external-dns source type you want to watch — `service`, `ingress`, `dnsendpoint`, or any of the Gateway API variants. The operator's reconciler watches the corresponding Kubernetes resources, extracts the endpoints that external-dns would manage, and writes them into the `DNSRecord` status. From there, they roll up into the parent `DNS` status.

Each endpoint in the status carries:
- `dnsName` — the FQDN
- `recordType` — A, AAAA, CNAME, etc.
- `targets` — the addresses it resolves to
- `syncStatus` — `sync`, `notavailable`, or `notsync` (the operator actually resolves the name and compares)
- `originRef` — the exact Service, Ingress, or other resource that produced it

**The `sreportal.io/portal` annotation** is the routing mechanism. Add it to any Service or Ingress and its FQDNs automatically appear under the named Portal in the web UI. No other configuration needed.

---

## In Practice

### Step 1 — Create a Portal

```yaml
apiVersion: sreportal.io/v1alpha1
kind: Portal
metadata:
  name: payments
  namespace: sreportal-system
spec:
  title: "Payments Platform"   # display name in the web UI
  subPath: payments             # accessible at /payments in the dashboard
  features:
    dns: true                   # enable DNS discovery for this portal
    alerts: true
```

### Step 2 — Create a DNS resource linked to the Portal

```yaml
apiVersion: sreportal.io/v1alpha1
kind: DNS
metadata:
  name: payments-dns
  namespace: sreportal-system
spec:
  portalRef: payments            # links to the Portal above

  # optional: static entries that aren't managed by external-dns
  groups:
    - name: "Legacy endpoints"
      description: "Hand-managed records pending migration"
      entries:
        - fqdn: legacy-api.payments.example.com
          description: "Old v1 API, decommission Q3"
```

### Step 3 — Create a DNSRecord to watch your Services

```yaml
apiVersion: sreportal.io/v1alpha1
kind: DNSRecord
metadata:
  name: payments-services
  namespace: sreportal-system
spec:
  # tells the operator which external-dns source type to scrape
  sourceType: service            # valid: service|ingress|dnsendpoint|istio-*|gateway-*
  portalRef: payments            # rolls up into the payments DNS status
```

From this point, any Service in the cluster that external-dns would pick up appears automatically in the `payments` Portal — no further configuration needed per service.

### Step 4 — Annotate your Services

To route a specific Service's FQDNs to a Portal, add the annotation:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: checkout-api
  namespace: payments
  annotations:
    external-dns.alpha.kubernetes.io/hostname: checkout.payments.example.com
    sreportal.io/portal: payments    # routes this service's FQDNs to the payments portal
spec:
  type: LoadBalancer
  selector:
    app: checkout-api
  ports:
    - port: 443
      targetPort: 8080
```

### What the operator writes back

After reconciliation, the `DNS` status contains a live view of all discovered FQDNs, grouped by source:

```yaml
status:
  groups:
    - name: payments-services       # auto-generated from the DNSRecord
      source: external-dns
      fqdns:
        - dnsName: checkout.payments.example.com
          recordType: A
          targets:
            - 203.0.113.42
          syncStatus: sync           # operator verified the DNS resolution matches
          lastSeen: "2026-05-14T01:23:00Z"
          originRef:
            kind: Service
            namespace: payments
            name: checkout-api
        - dnsName: api.payments.example.com
          recordType: CNAME
          targets:
            - alb-1234.us-east-1.elb.amazonaws.com
          syncStatus: notsync        # DNS resolves to a different target — worth investigating
          lastSeen: "2026-05-14T01:23:00Z"
          originRef:
            kind: Service
            namespace: payments
            name: public-api
    - name: Legacy endpoints
      source: manual
      fqdns:
        - dnsName: legacy-api.payments.example.com
          syncStatus: notavailable   # FQDN doesn't resolve — decommissioned?
```

The `syncStatus` field is not cosmetic. The operator performs an actual DNS lookup on each FQDN and compares the result against the expected type and targets. `notsync` means the record exists but points somewhere unexpected. `notavailable` means it doesn't resolve at all. These states surface directly in the web UI.

### The web UI Links page

The operator runs a single binary that includes the web server. Once deployed, you hit its HTTP endpoint and get a React dashboard. The Links page shows every Portal as a section, with each FQDN listed under its group. Sync status is color-coded. The `originRef` appears as a clickable reference — you can see immediately which Service or Ingress produced a given hostname without touching `kubectl`.

No separate frontend deployment, no additional database, no scraper job to schedule. The operator is the source of truth.

---

## Why It Matters

**Faster incident response.** The FQDN, its current DNS target, and the resource that produced it are all in one place. The 20-minute "which hostname are we actually on?" conversation becomes a 10-second lookup.

**Drift detection.** `notsync` entries mean your cluster state and your DNS provider are out of sync. Previously you'd discover this when a customer reported an outage. Now you see it before the alert fires.

**Ownership without a spreadsheet.** The `originRef` field traces every FQDN back to a specific Kubernetes resource, in a specific namespace. Teams stop asking "whose is this?" because the data is already there.

**No extra tooling.** The inventory lives in the cluster. It's queryable with `kubectl`, available via Connect gRPC, and exposed through an MCP server — meaning AI agents and internal tools can query DNS state programmatically using the same API.

---

## What's Next

Now you can see what's exposed. Episode 2 covers the other side of that picture: the alerts that fire on those services. We'll look at how SRE Portal's Alertmanager integration pulls active alerts from your Alertmanager instances and surfaces them per Portal, so you can correlate "this FQDN is down" with "these alerts are firing" without switching tools.

**Source code:** [github.com/golgoth31/sreportal](https://github.com/golgoth31/sreportal)

---

*Built together with [Benjamin Colmart](https://github.com/benjamincolmart).*
