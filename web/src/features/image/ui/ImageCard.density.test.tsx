import { cleanup, render, within } from "@testing-library/react";
import type { ReactElement } from "react";
import { afterEach, describe, expect, it } from "vitest";

import { TooltipProvider } from "@/components/ui/tooltip";
import type { Image } from "../domain/image.types";
import { ImageCard } from "./ImageCard";
import { ImageGroupList } from "./ImageGroupList";

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

const imageNoUpgrade: Image = {
  ...image,
  tag: "2.0.0",
  upgradeAvailable: false,
  latestVersion: "2.0.0",
};

const imageUnchecked: Image = {
  ...image,
  tag: "0.1.0",
  upgradeAvailable: false,
  latestVersion: undefined,
};

const imageOtherRegistry: Image = {
  ...image,
  registry: "docker.io",
  repository: "library/nginx",
  tag: "1.25",
};

function renderWithTooltip(ui: ReactElement) {
  return render(<TooltipProvider>{ui}</TooltipProvider>);
}

afterEach(() => {
  cleanup();
});

describe("Image dense Soft Material table", () => {
  it("uses one page-level headerless table across groups", () => {
    const { container } = renderWithTooltip(
      <ImageGroupList
        groups={[
          {
            registry: "ghcr.io",
            images: [image],
            stats: { total: 1, upgrades: 1, mutated: 0, injected: 0 },
          },
          {
            registry: "docker.io",
            images: [imageOtherRegistry],
            stats: { total: 1, upgrades: 1, mutated: 0, injected: 0 },
          },
        ]}
        isLoading={false}
        hasFilters={false}
        onClearFilters={() => {}}
      />,
    );
    expect(within(container).getAllByRole("table")).toHaveLength(1);
    const table = within(container).getByRole("table");
    expect(within(table).queryByRole("columnheader")).toBeNull();
    expect(table.className).toMatch(/table-fixed/);
    expect(
      within(table).getByRole("button", { name: /Copy ghcr\.io\/org\/api:1\.2\.3/ }).className,
    ).toMatch(/font-mono/);
  });

  it("shows upgrade version in the status column and on the upgrade icon", () => {
    const { container } = renderWithTooltip(
      <ImageGroupList
        groups={[
          {
            registry: "ghcr.io",
            images: [image],
            stats: { total: 1, upgrades: 1, mutated: 0, injected: 0 },
          },
        ]}
        isLoading={false}
        hasFilters={false}
        onClearFilters={() => {}}
      />,
    );
    const table = within(container).getByRole("table");
    expect(
      within(table).getByLabelText("Upgrade available: 1.3.0"),
    ).toBeInTheDocument();
    const version = within(table).getByText("1.3.0");
    expect(version.className).toMatch(/text-emerald/);
  });

  it("always shows a status label, including when unchecked", () => {
    const { container } = renderWithTooltip(
      <ImageGroupList
        groups={[
          {
            registry: "ghcr.io",
            images: [imageUnchecked, imageNoUpgrade],
            stats: { total: 2, upgrades: 0, mutated: 0, injected: 0 },
          },
        ]}
        isLoading={false}
        hasFilters={false}
        onClearFilters={() => {}}
      />,
    );
    const table = within(container).getByRole("table");
    expect(within(table).getByText("Not checked")).toBeInTheDocument();
    expect(within(table).getByText("No upgrade available")).toBeInTheDocument();
    expect(within(table).queryByLabelText(/Upgrade available/)).toBeNull();
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
