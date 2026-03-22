import { CheckIcon, CopyIcon, PlugIcon } from "lucide-react";
import { useCallback } from "react";

import { PageRefreshButton } from "@/components/PageRefreshButton";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { useCopyToClipboard } from "@/hooks/useCopyToClipboard";

interface McpTool {
  name: string;
  description: string;
  filters: string[];
}

const MCP_DNS_TOOLS: McpTool[] = [
  {
    name: "search_fqdns",
    description:
      "Search for FQDNs (Fully Qualified Domain Names) in the SRE Portal. Returns a list of DNS entries matching the search criteria.",
    filters: ["query", "source", "group", "portal", "namespace"],
  },
  {
    name: "list_portals",
    description:
      "List all available portals in the SRE Portal. Portals are entry points that group DNS entries together.",
    filters: [],
  },
  {
    name: "get_fqdn_details",
    description:
      "Get detailed information about a specific FQDN. Returns the full DNS record details including targets, record type, and metadata.",
    filters: ["fqdn"],
  },
];

const MCP_ALERTS_TOOLS: McpTool[] = [
  {
    name: "list_alerts",
    description:
      "List active alerts from Alertmanager resources. Returns Alertmanager resources with their active alerts and labels.",
    filters: ["portal", "namespace", "search", "state"],
  },
];

const MCP_METRICS_TOOLS: McpTool[] = [
  {
    name: "list_metrics",
    description:
      "List current values of SRE Portal Prometheus metrics (sreportal_* custom metrics with labels and types).",
    filters: ["subsystem", "search"],
  },
];

const MCP_RELEASES_TOOLS: McpTool[] = [
  {
    name: "list_releases",
    description:
      "List release entries for a day with previous/next day hints for navigation.",
    filters: ["day"],
  },
];

interface McpEndpointSection {
  id: string;
  title: string;
  path: string;
  note?: string;
  tools: McpTool[];
}

const MCP_SECTIONS: McpEndpointSection[] = [
  {
    id: "dns",
    title: "DNS & Portals",
    path: "/mcp/dns",
    note: "The same server is also mounted at /mcp for backward compatibility.",
    tools: MCP_DNS_TOOLS,
  },
  {
    id: "alerts",
    title: "Alerts",
    path: "/mcp/alerts",
    tools: MCP_ALERTS_TOOLS,
  },
  {
    id: "metrics",
    title: "Metrics",
    path: "/mcp/metrics",
    tools: MCP_METRICS_TOOLS,
  },
  {
    id: "releases",
    title: "Releases",
    path: "/mcp/releases",
    tools: MCP_RELEASES_TOOLS,
  },
];

function CopyableCode({
  id,
  value,
  label,
}: {
  id: string;
  value: string;
  label: string;
}) {
  const { copied, copy } = useCopyToClipboard(value);

  return (
    <div className="relative rounded-md bg-muted border">
      <Button
        variant="ghost"
        size="icon"
        className="absolute right-2 top-2 size-7 z-10"
        onClick={copy}
        aria-label={`Copy ${label}`}
      >
        {copied ? (
          <CheckIcon className="size-4 text-green-600" />
        ) : (
          <CopyIcon className="size-4" />
        )}
      </Button>
      <pre
        id={id}
        className="overflow-x-auto p-4 pr-10 text-xs font-mono leading-relaxed"
      >
        <code>{value}</code>
      </pre>
    </div>
  );
}

function McpToolsTable({ tools }: { tools: McpTool[] }) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead className="w-48">Tool</TableHead>
          <TableHead>Description &amp; Filters</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {tools.map((tool) => (
          <TableRow key={tool.name}>
            <TableCell className="align-top">
              <code className="font-mono text-xs font-semibold text-primary">
                {tool.name}
              </code>
            </TableCell>
            <TableCell className="space-y-2">
              <p className="text-muted-foreground text-sm leading-relaxed">
                {tool.description}
              </p>
              {tool.filters.length > 0 && (
                <div className="flex flex-wrap gap-1">
                  {tool.filters.map((f) => (
                    <Badge
                      key={f}
                      variant="outline"
                      className="font-mono text-xs"
                    >
                      {f}
                    </Badge>
                  ))}
                </div>
              )}
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}

function sectionUrl(baseUrl: string, path: string): string {
  return `${baseUrl}${path}`;
}

export function McpPage() {
  const handleRefresh = useCallback(() => {
    window.location.reload();
  }, []);

  const baseUrl = window.location.origin;

  const mcpRootUrl = sectionUrl(baseUrl, "/mcp");
  const mcpDnsUrl = sectionUrl(baseUrl, "/mcp/dns");

  const claudeDesktopConfig = JSON.stringify(
    {
      mcpServers: Object.fromEntries(
        MCP_SECTIONS.map((s) => [
          `sreportal-${s.id}`,
          { url: sectionUrl(baseUrl, s.path) },
        ]),
      ),
    },
    null,
    2,
  );

  const claudeCodeCommands = MCP_SECTIONS.map(
    (s) =>
      `claude mcp add sreportal-${s.id} --transport http ${sectionUrl(baseUrl, s.path)}`,
  ).join("\n");

  return (
    <div className="max-w-3xl mx-auto py-8 px-4 space-y-8">
      {/* Header */}
      <div className="flex items-start justify-between gap-4 flex-wrap">
        <div className="flex items-center gap-3 min-w-0">
          <PlugIcon className="size-6 text-primary shrink-0" />
          <div>
            <h1 className="text-2xl font-bold tracking-tight">
              MCP Server Integration
            </h1>
            <p className="text-muted-foreground text-sm mt-1">
              Connect AI assistants directly to your SRE Portal via the Model
              Context Protocol (Streamable HTTP). Requires the operator to be
              started with <code className="font-mono text-xs">--enable-mcp</code>
              .
            </p>
          </div>
        </div>
        <PageRefreshButton
          className="shrink-0"
          onRefresh={handleRefresh}
          label="Reload page"
        />
      </div>

      <Separator />

      {MCP_SECTIONS.map((section) => (
        <section key={section.id} className="space-y-4">
          <div>
            <h2 className="text-lg font-semibold">
              {section.title} — {section.path}
            </h2>
            {section.note && (
              <p className="text-muted-foreground text-xs mt-1">{section.note}</p>
            )}
          </div>
          <McpToolsTable tools={section.tools} />
        </section>
      ))}

      <Separator />

      {/* Claude Desktop */}
      <section className="space-y-3">
        <h2 className="text-lg font-semibold">Claude Desktop</h2>
        <p className="text-muted-foreground text-sm">
          Add the following to your{" "}
          <code className="font-mono text-xs bg-muted border rounded px-1 py-0.5">
            claude_desktop_config.json
          </code>
          :
        </p>
        <CopyableCode
          id="claude-desktop-config"
          value={claudeDesktopConfig}
          label="Claude Desktop config"
        />
      </section>

      {/* Claude Code */}
      <section className="space-y-3">
        <h2 className="text-lg font-semibold">Claude Code</h2>
        <p className="text-muted-foreground text-sm">
          Run these commands to register all MCP servers:
        </p>
        <CopyableCode
          id="claude-code-command"
          value={claudeCodeCommands}
          label="Claude Code commands"
        />
      </section>

      {/* Cursor */}
      <section className="space-y-3">
        <h2 className="text-lg font-semibold">Cursor</h2>
        <p className="text-muted-foreground text-sm">
          In Cursor settings, add MCP servers with type{" "}
          <code className="font-mono text-xs bg-muted border rounded px-1 py-0.5">
            http
          </code>{" "}
          and these URLs:
        </p>
        <div className="space-y-2">
          {MCP_SECTIONS.map((section) => (
            <CopyableCode
              key={section.id}
              id={`cursor-endpoint-${section.id}`}
              value={sectionUrl(baseUrl, section.path)}
              label={`MCP ${section.title} endpoint URL`}
            />
          ))}
        </div>
      </section>

      <Separator />

      {/* MCP endpoints summary */}
      <section className="space-y-2">
        <p className="text-muted-foreground text-xs">
          Legacy DNS alias:{" "}
          <code className="font-mono text-xs bg-muted border rounded px-1 py-0.5">
            {mcpRootUrl}
          </code>{" "}
          (same tools as {mcpDnsUrl})
        </p>
        {MCP_SECTIONS.map((section) => (
          <p key={section.id} className="text-muted-foreground text-xs">
            {section.title}:{" "}
            <code className="font-mono text-xs bg-muted border rounded px-1 py-0.5">
              {sectionUrl(baseUrl, section.path)}
            </code>
          </p>
        ))}
      </section>
    </div>
  );
}
