import { CheckIcon, CopyIcon, PlugIcon } from "lucide-react";
import { useCallback, useState } from "react";
import { toast } from "sonner";

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

interface McpTool {
  name: string;
  description: string;
  filters: string[];
}

const MCP_TOOLS: McpTool[] = [
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

function CopyableCode({
  id,
  value,
  label,
}: {
  id: string;
  value: string;
  label: string;
}) {
  const [copied, setCopied] = useState(false);

  const handleCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      toast.success("Copied to clipboard");
      setTimeout(() => setCopied(false), 2000);
    } catch {
      toast.error("Failed to copy");
    }
  }, [value]);

  return (
    <div className="relative rounded-md bg-muted border">
      <Button
        variant="ghost"
        size="icon"
        className="absolute right-2 top-2 size-7 z-10"
        onClick={handleCopy}
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

export function McpPage() {
  const mcpEndpoint = `${window.location.origin}/mcp`;

  const claudeDesktopConfig = JSON.stringify(
    { mcpServers: { sreportal: { url: mcpEndpoint } } },
    null,
    2
  );
  const claudeCodeCommand = `claude mcp add sreportal --transport http ${mcpEndpoint}`;

  return (
    <div className="max-w-3xl mx-auto py-8 px-4 space-y-8">
      {/* Header */}
      <div className="flex items-center gap-3">
        <PlugIcon className="size-6 text-primary" />
        <div>
          <h1 className="text-2xl font-bold tracking-tight">
            MCP Server Integration
          </h1>
          <p className="text-muted-foreground text-sm mt-1">
            Connect AI assistants directly to your SRE Portal via the Model
            Context Protocol.
          </p>
        </div>
      </div>

      <Separator />

      {/* Tools table */}
      <section className="space-y-4">
        <h2 className="text-lg font-semibold">Available Tools</h2>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-48">Tool</TableHead>
              <TableHead>Description &amp; Filters</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {MCP_TOOLS.map((tool) => (
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
      </section>

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
          Run this command to register the MCP server:
        </p>
        <CopyableCode
          id="claude-code-command"
          value={claudeCodeCommand}
          label="Claude Code command"
        />
      </section>

      {/* Cursor */}
      <section className="space-y-3">
        <h2 className="text-lg font-semibold">Cursor</h2>
        <p className="text-muted-foreground text-sm">
          In Cursor settings, add an MCP server with type{" "}
          <code className="font-mono text-xs bg-muted border rounded px-1 py-0.5">
            http
          </code>{" "}
          and URL:
        </p>
        <CopyableCode
          id="cursor-endpoint"
          value={mcpEndpoint}
          label="MCP endpoint URL"
        />
      </section>

      <Separator />

      {/* MCP endpoint */}
      <section className="space-y-2">
        <p className="text-muted-foreground text-xs">
          MCP endpoint:{" "}
          <code className="font-mono text-xs bg-muted border rounded px-1 py-0.5">
            {mcpEndpoint}
          </code>
        </p>
      </section>
    </div>
  );
}
