import { useParams, Link } from "react-router-dom";
import { useRun, useRunTelemetry } from "@/hooks/use-api";
import { StatusBadge } from "@/components/status-badge";
import {
  Card,
  CardHeader,
  CardTitle,
  CardContent,
} from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { ArrowLeft, Clock, Cpu, Zap } from "lucide-react";
import { formatAge } from "@/lib/utils";

export function RunDetailPage() {
  const { name } = useParams<{ name: string }>();
  const { data: run, isLoading } = useRun(name || "");
  const telemetry = useRunTelemetry(name || "");

  if (isLoading) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-8 w-64" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }

  if (!run) {
    return <p className="text-muted-foreground">Run not found</p>;
  }

  const usage = run.status?.tokenUsage;
  const duration = usage?.durationMs
    ? `${(usage.durationMs / 1000).toFixed(1)}s`
    : "—";

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <Link to="/runs" className="text-muted-foreground hover:text-foreground">
          <ArrowLeft className="h-5 w-5" />
        </Link>
        <div>
          <h1 className="text-xl font-bold font-mono">{run.metadata.name}</h1>
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <Link to={`/instances/${run.spec.instanceRef}`} className="hover:text-primary">
              {run.spec.instanceRef}
            </Link>
            <span>·</span>
            <StatusBadge phase={run.status?.phase} />
            <span>·</span>
            {formatAge(run.metadata.creationTimestamp)} ago
          </div>
        </div>
      </div>

      {/* Stats row */}
      {usage && (
        <div className="grid gap-4 sm:grid-cols-4">
          <Card>
            <CardContent className="flex items-center gap-3 p-4">
              <Zap className="h-5 w-5 text-amber-400" />
              <div>
                <p className="text-sm text-muted-foreground">Total Tokens</p>
                <p className="text-lg font-bold">
                  {usage.totalTokens.toLocaleString()}
                </p>
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="flex items-center gap-3 p-4">
              <Cpu className="h-5 w-5 text-blue-400" />
              <div>
                <p className="text-sm text-muted-foreground">Tool Calls</p>
                <p className="text-lg font-bold">{usage.toolCalls}</p>
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="flex items-center gap-3 p-4">
              <Clock className="h-5 w-5 text-purple-400" />
              <div>
                <p className="text-sm text-muted-foreground">Duration</p>
                <p className="text-lg font-bold">{duration}</p>
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="flex items-center gap-3 p-4">
              <div>
                <p className="text-sm text-muted-foreground">In / Out</p>
                <p className="text-sm font-mono">
                  {usage.inputTokens.toLocaleString()} /{" "}
                  {usage.outputTokens.toLocaleString()}
                </p>
              </div>
            </CardContent>
          </Card>
        </div>
      )}

      <Tabs defaultValue="task">
        <TabsList>
          <TabsTrigger value="task">Task</TabsTrigger>
          <TabsTrigger value="result">Result</TabsTrigger>
          <TabsTrigger value="telemetry">Telemetry</TabsTrigger>
          <TabsTrigger value="spec">Spec</TabsTrigger>
        </TabsList>

        <TabsContent value="task">
          <Card>
            <CardContent className="pt-6">
              <pre className="whitespace-pre-wrap text-sm">{run.spec.task}</pre>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="result">
          <Card>
            <CardContent className="pt-6">
              {run.status?.result ? (
                <pre className="whitespace-pre-wrap text-sm">
                  {run.status.result}
                </pre>
              ) : run.status?.error ? (
                <div className="space-y-2">
                  <Badge variant="destructive">Error</Badge>
                  <pre className="whitespace-pre-wrap text-sm text-destructive">
                    {run.status.error}
                  </pre>
                </div>
              ) : (
                <p className="text-sm text-muted-foreground">
                  {run.status?.phase === "Running"
                    ? "Run is still in progress…"
                    : "No result available"}
                </p>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="telemetry">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Trace Correlation</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              {!telemetry.data ? (
                <p className="text-sm text-muted-foreground">
                  Telemetry not available yet. If this is a fresh run, wait a few seconds.
                </p>
              ) : (
                <>
                  <div className="space-y-2">
                    <p className="text-xs uppercase tracking-wide text-muted-foreground">
                      Trace IDs
                    </p>
                    {telemetry.data.traceIds?.length ? (
                      <div className="space-y-1">
                        {telemetry.data.traceIds.map((id) => (
                          <div key={id} className="font-mono text-xs rounded bg-muted/40 p-2">
                            {id}
                          </div>
                        ))}
                      </div>
                    ) : (
                      <p className="text-sm text-muted-foreground">No trace IDs found in logs yet.</p>
                    )}
                  </div>

                  <div className="grid gap-4 sm:grid-cols-2">
                    <div className="space-y-2">
                      <p className="text-xs uppercase tracking-wide text-muted-foreground">Spans Seen</p>
                      {telemetry.data.spanNames?.length ? (
                        <div className="flex flex-wrap gap-2">
                          {telemetry.data.spanNames.map((span) => (
                            <Badge key={span} variant="secondary">{span}</Badge>
                          ))}
                        </div>
                      ) : (
                        <p className="text-sm text-muted-foreground">No collector span evidence yet.</p>
                      )}
                    </div>
                    <div className="space-y-2">
                      <p className="text-xs uppercase tracking-wide text-muted-foreground">Metrics Seen</p>
                      {telemetry.data.metricNames?.length ? (
                        <div className="flex flex-wrap gap-2">
                          {telemetry.data.metricNames.map((metric) => (
                            <Badge key={metric} variant="outline">{metric}</Badge>
                          ))}
                        </div>
                      ) : (
                        <p className="text-sm text-muted-foreground">No collector metric evidence yet.</p>
                      )}
                    </div>
                  </div>

                  <Separator />

                  <div className="space-y-2">
                    <p className="text-xs uppercase tracking-wide text-muted-foreground">
                      Recent Trace Events
                    </p>
                    {telemetry.data.events?.length ? (
                      <div className="max-h-80 overflow-auto space-y-2">
                        {telemetry.data.events.slice(-25).map((ev, i) => (
                          <div key={`${ev.time || i}-${ev.spanId || i}`} className="rounded border p-2 text-xs">
                            <div className="flex flex-wrap items-center gap-2 text-muted-foreground">
                              <span>{ev.time || "—"}</span>
                              {ev.level && <Badge variant="outline">{ev.level}</Badge>}
                              {ev.spanId && <span className="font-mono">span={ev.spanId}</span>}
                            </div>
                            <p className="mt-1">{ev.message || "event"}</p>
                          </div>
                        ))}
                      </div>
                    ) : (
                      <p className="text-sm text-muted-foreground">No structured trace events found in agent logs.</p>
                    )}
                  </div>
                </>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="spec">
          <Card>
            <CardContent className="pt-6">
              <pre className="text-xs font-mono whitespace-pre-wrap rounded bg-muted/50 p-4 overflow-auto max-h-96">
                {JSON.stringify(run.spec, null, 2)}
              </pre>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>

      {/* Pod info */}
      {run.status?.podName && (
        <>
          <Separator />
          <div className="text-sm text-muted-foreground">
            Pod: <span className="font-mono">{run.status.podName}</span>
            {run.status.exitCode !== undefined && (
              <>
                {" "}
                · Exit code:{" "}
                <span className="font-mono">{run.status.exitCode}</span>
              </>
            )}
          </div>
        </>
      )}
    </div>
  );
}
