import { create } from "@bufbuild/protobuf";
import { createClient } from "@connectrpc/connect";
import { createGrpcWebTransport } from "@connectrpc/connect-web";

import {
  ListMetricsRequestSchema,
  MetricsService,
  type HistogramBucket as ProtoHistogramBucket,
  type HistogramValue as ProtoHistogramValue,
  type Metric as ProtoMetric,
  type MetricFamily as ProtoMetricFamily,
} from "@/gen/sreportal/v1/metrics_pb";
import type {
  HistogramBucket,
  HistogramValue,
  Metric,
  MetricFamily,
  MetricType,
} from "../domain/metrics.types";

const transport = createGrpcWebTransport({ baseUrl: window.location.origin });
const client = createClient(MetricsService, transport);

function toDomainBucket(b: ProtoHistogramBucket): HistogramBucket {
  return {
    cumulativeCount: Number(b.cumulativeCount),
    upperBound: b.upperBound,
  };
}

function toDomainHistogram(
  h: ProtoHistogramValue | undefined,
): HistogramValue | undefined {
  if (h == null) return undefined;
  return {
    sampleCount: Number(h.sampleCount),
    sampleSum: h.sampleSum,
    buckets: h.buckets.map(toDomainBucket),
  };
}

function toDomainMetric(m: ProtoMetric): Metric {
  return {
    labels: { ...m.labels },
    value: m.value,
    histogram: toDomainHistogram(m.histogram),
  };
}

function toDomainFamily(f: ProtoMetricFamily): MetricFamily {
  return {
    name: f.name,
    help: f.help,
    type: f.type as MetricType,
    metrics: f.metrics.map(toDomainMetric),
  };
}

export interface ListMetricsParams {
  subsystem?: string;
  search?: string;
}

export async function listMetrics(
  params: ListMetricsParams = {},
): Promise<MetricFamily[]> {
  const request = create(ListMetricsRequestSchema, {
    subsystem: params.subsystem ?? "",
    search: params.search ?? "",
  });
  const response = await client.listMetrics(request);
  return response.families.map(toDomainFamily);
}
