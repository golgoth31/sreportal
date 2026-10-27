import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "./table";

describe("table Soft Material density", () => {
  it("uses density row tokens on head and tighter cell padding", () => {
    const { container } = render(
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>State</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow>
            <TableCell>active</TableCell>
          </TableRow>
        </TableBody>
      </Table>,
    );

    const head = container.querySelector('[data-slot="table-head"]');
    const cell = container.querySelector('[data-slot="table-cell"]');
    expect(head?.className).toMatch(/h-\[var\(--row-height-tight\)\]/);
    expect(head?.className).not.toMatch(/\bh-10\b/);
    expect(cell?.className).toMatch(/\bpy-1\b/);
    expect(cell?.className).toMatch(/\bpx-2\b/);
  });
});
