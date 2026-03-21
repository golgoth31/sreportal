import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { ThemeToggle } from "@/components/ThemeToggle";
import { TooltipProvider } from "@/components/ui/tooltip";

const mockUseTheme = vi.fn();

vi.mock("@/hooks/useTheme", () => ({
  useTheme: () => mockUseTheme(),
}));

function renderToggle() {
  return render(
    <TooltipProvider>
      <ThemeToggle />
    </TooltipProvider>,
  );
}

describe("ThemeToggle", () => {
  it("when mode is light invokes toggle when the button is activated", async () => {
    const toggle = vi.fn();
    mockUseTheme.mockReturnValue({ mode: "light", toggle });
    const user = userEvent.setup();
    renderToggle();

    await user.click(
      screen.getByRole("button", { name: /light theme \(click for dark\)/i }),
    );

    expect(toggle).toHaveBeenCalledTimes(1);
  });

  it("when mode is dark exposes the dark theme label", () => {
    mockUseTheme.mockReturnValue({ mode: "dark", toggle: vi.fn() });
    renderToggle();

    expect(
      screen.getByRole("button", {
        name: /dark theme \(click for system\)/i,
      }),
    ).toBeInTheDocument();
  });
});
