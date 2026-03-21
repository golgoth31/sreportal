import { Bar, BarChart, CartesianGrid, XAxis } from "recharts";

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from "@/components/ui/chart";

import type { FqdnBarDataPoint } from "../domain/dashboard.types";

interface FqdnBarChartProps {
  readonly data: readonly FqdnBarDataPoint[];
}

const chartConfig = {
  value: {
    label: "FQDNs",
    color: "var(--chart-1)",
  },
} satisfies ChartConfig;

export function FqdnBarChart({ data }: FqdnBarChartProps) {
  if (data.length === 0) {
    return (
      <Card className="flex flex-col">
        <CardHeader>
          <CardTitle>FQDNs by Portal</CardTitle>
          <CardDescription>No data available</CardDescription>
        </CardHeader>
      </Card>
    );
  }

  return (
    <Card className="flex flex-col">
      <CardHeader>
        <CardTitle>FQDNs by Portal</CardTitle>
        <CardDescription>Distribution by portal and source</CardDescription>
      </CardHeader>
      <CardContent>
        <ChartContainer config={chartConfig}>
          <BarChart accessibilityLayer data={[...data]}>
            <CartesianGrid vertical={false} />
            <XAxis
              dataKey="label"
              tickLine={false}
              tickMargin={10}
              axisLine={false}
            />
            <ChartTooltip
              cursor={false}
              content={<ChartTooltipContent hideLabel />}
            />
            <Bar dataKey="value" fill="var(--color-value)" radius={8} />
          </BarChart>
        </ChartContainer>
      </CardContent>
    </Card>
  );
}
