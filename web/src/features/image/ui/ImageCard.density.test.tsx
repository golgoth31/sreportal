import { cleanup, render, within } from "@testing-library/react";
import type { ReactElement } from "react";
import { afterEach, describe, expect, it } from "vitest";

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

afterEach(() => {
  cleanup();
});

describe("Image dense Soft Material table", () => {
  it("renders aligned table columns for a group", () => {
    const { container } = renderWithTooltip(
      <ImageGroupCard
        group={{
          registry: "ghcr.io",
          images: [image],
          stats: { total: 1, upgrades: 1, mutated: 0, injected: 0 },
        }}
      />,
    );
    const table = within(container).getByRole("table");
    expect(within(table).getByRole("columnheader", { name: "Name" })).toBeInTheDocument();
    expect(within(table).getByRole("columnheader", { name: "Image" })).toBeInTheDocument();
    expect(within(table).getByRole("columnheader", { name: "Type" })).toBeInTheDocument();
    expect(
      within(table).getByRole("button", { name: /Copy ghcr\.io\/org\/api:1\.2\.3/ }).className,
    ).toMatch(/font-mono/);
    expect(within(table).getByText("semver").className).toMatch(/font-mono/);
  });

  it("shows upgrade version beside the image ref and on the upgrade icon", () => {
    const { container } = renderWithTooltip(
      <ImageGroupCard
        group={{
          registry: "ghcr.io",
          images: [image],
          stats: { total: 1, upgrades: 1, mutated: 0, injected: 0 },
        }}
      />,
    );
    const table = within(container).getByRole("table");
    expect(
      within(table).getByLabelText("Upgrade available: 1.3.0"),
    ).toBeInTheDocument();
    expect(within(table).getByText("1.3.0")).toBeInTheDocument();
  });

  it("renders a table row for a single image when wrapped in a table body", () => {
    const { container } = renderWithTooltip(
      <table>
        <tbody>
          <ImageCard image={image} />
        </tbody>
      </table>,
    );
    expect(container.querySelector("tr")).toBeTruthy();
    expect(container.querySelectorAll("td").length).toBeGreaterThanOrEqual(6);
  });
});
