import { render, screen } from "@testing-library/react";
import type { ReactElement } from "react";
import { describe, expect, it } from "vitest";

import { TooltipProvider } from "@/components/ui/tooltip";
import type { Image } from "../domain/image.types";
import { ImageCard } from "./ImageCard";
import { ImageGroupCard } from "./ImageGroupCard";

const image: Image = {
  registry: "ghcr.io",
  repository: "org/api",
  tag: "1.2.3",
  tagType: "semver",
  workloads: [
    {
      kind: "Deployment",
      namespace: "default",
      name: "api",
      container: "api",
      source: "spec",
    },
  ],
  upgradeAvailable: true,
  latestVersion: "1.3.0",
};

function renderWithTooltip(ui: ReactElement) {
  return render(<TooltipProvider>{ui}</TooltipProvider>);
}

describe("Image dense Soft Material rows", () => {
  it("renders a dense mono row shell", () => {
    const { container } = renderWithTooltip(<ImageCard image={image} />);
    const root = container.firstElementChild as HTMLElement;
    expect(root.className).toMatch(/min-h-\[var\(--row-height\)\]/);
    expect(root.className).toMatch(/flex-row/);
    expect(
      screen.getByRole("button", { name: /Copy ghcr\.io\/org\/api:1\.2\.3/ }).className,
    ).toMatch(/font-mono/);
    expect(screen.getByText("semver").className).toMatch(/font-mono/);
  });

  it("lists images in a single column (no multi-col grid)", () => {
    const { container } = renderWithTooltip(
      <ImageGroupCard
        group={{
          registry: "ghcr.io",
          images: [image],
          stats: { total: 1, upgrades: 1, mutated: 0, injected: 0 },
        }}
      />,
    );
    const grid = container.querySelector(".border-t");
    expect(grid?.className).not.toMatch(/sm:grid-cols-2/);
    expect(grid?.className).not.toMatch(/lg:grid-cols-3/);
    expect(grid?.className).toMatch(/gap-\[var\(--density-gap\)\]/);
  });
});
