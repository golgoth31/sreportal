import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";

describe("soft material chrome", () => {
  it("button root does not use translate-y press motion", () => {
    const { container } = render(<Button>Save</Button>);
    const btn = container.querySelector('[data-slot="button"]');
    expect(btn?.className).not.toMatch(/active:translate-y-px/);
  });

  it("card uses elevation shadow token class and not foreground ring", () => {
    const { container } = render(<Card>Body</Card>);
    const card = container.querySelector('[data-slot="card"]');
    expect(card?.className).toMatch(/shadow-\[var\(--elevation-1\)\]/);
    expect(card?.className).not.toMatch(/ring-1/);
  });

  it("badge is not fully pill-rounded", () => {
    const { container } = render(<Badge>ok</Badge>);
    const el = container.querySelector('[data-slot="badge"]');
    expect(el?.className).not.toMatch(/rounded-4xl/);
    expect(el?.className).toMatch(/rounded-md/);
  });

  it("input uses rounded-md", () => {
    const { container } = render(<Input aria-label="q" />);
    const el = container.querySelector('[data-slot="input"]');
    expect(el?.className).toMatch(/rounded-md/);
    expect(el?.className).not.toMatch(/rounded-lg/);
  });
});
