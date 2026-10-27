import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";

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
});
