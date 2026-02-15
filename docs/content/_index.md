---
title: SRE Portal
layout: hextra-home
---

{{< hextra/hero-badge >}}
  <div class="hx-w-2 hx-h-2 hx-rounded-full hx-bg-primary-400"></div>
  <span>Free, open source</span>
  {{< icon name="arrow-circle-right" attributes="height=14" >}}
{{< /hextra/hero-badge >}}

<div class="hx-mt-6 hx-mb-6">
{{< hextra/hero-headline >}}
  Kubernetes Operator for&nbsp;<br class="sm:hx-block hx-hidden" />Service Status Pages &amp; DNS Discovery
{{< /hextra/hero-headline >}}
</div>

<div class="hx-mb-12">
{{< hextra/hero-subtitle >}}
  Discover, aggregate, and visualize DNS records across your Kubernetes clusters&nbsp;<br class="sm:hx-block hx-hidden" />with a unified web dashboard.
{{< /hextra/hero-subtitle >}}
</div>

<div class="hx-mb-6">
{{< hextra/hero-button text="Get Started" link="docs/getting-started" >}}
</div>

<div class="hx-mt-6"></div>

{{< hextra/feature-grid >}}
  {{< hextra/feature-card
    title="DNS Discovery"
    subtitle="Automatically discover DNS records from Services, Ingresses, Istio Gateways, and external-dns endpoints across all namespaces."
    class="hx-aspect-auto md:hx-aspect-[1.1/1] max-md:hx-min-h-[340px]"
    style="background: radial-gradient(ellipse at 50% 80%,rgba(59,130,246,0.15),hsla(0,0%,100%,0));"
  >}}
  {{< hextra/feature-card
    title="Portal Routing"
    subtitle="Organize endpoints into multiple portals using simple Kubernetes annotations. Each portal gets its own view in the web dashboard."
    class="hx-aspect-auto md:hx-aspect-[1.1/1] max-lg:hx-min-h-[340px]"
    style="background: radial-gradient(ellipse at 50% 80%,rgba(142,53,74,0.15),hsla(0,0%,100%,0));"
  >}}
  {{< hextra/feature-card
    title="Web Dashboard"
    subtitle="Angular-powered SPA served directly by the operator. Browse FQDNs grouped by source, filter by search, and navigate between portals."
    class="hx-aspect-auto md:hx-aspect-[1.1/1] max-md:hx-min-h-[340px]"
    style="background: radial-gradient(ellipse at 50% 80%,rgba(34,197,94,0.15),hsla(0,0%,100%,0));"
  >}}
  {{< hextra/feature-card
    title="Connect API"
    subtitle="gRPC-compatible Connect protocol API for listing and streaming FQDN updates in real time."
  >}}
  {{< hextra/feature-card
    title="Flexible Grouping"
    subtitle="Group FQDNs by annotation, label, namespace, or custom rules. Combine automatic discovery with manual DNS entries."
  >}}
  {{< hextra/feature-card
    title="Single Container"
    subtitle="Controller, gRPC API, and web UI all run in a single container for simple deployment and low resource footprint."
  >}}
{{< /hextra/feature-grid >}}
