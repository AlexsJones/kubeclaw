import { useState } from "react";
import { Link } from "react-router-dom";
import { usePersonaPacks } from "@/hooks/use-api";
import { StatusBadge } from "@/components/status-badge";
import {
  Table,
  TableHeader,
  TableRow,
  TableHead,
  TableBody,
  TableCell,
} from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { Input } from "@/components/ui/input";
import { ExternalLink } from "lucide-react";
import { formatAge } from "@/lib/utils";

export function PersonasPage() {
  const { data, isLoading } = usePersonaPacks();
  const [search, setSearch] = useState("");

  const filtered = (data || []).filter((p) =>
    p.metadata.name.toLowerCase().includes(search.toLowerCase())
  );

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-2xl font-bold">Persona Packs</h1>
        <p className="text-sm text-muted-foreground">
          Pre-configured agent bundles — stamps out Instances, Schedules, and
          memory automatically
        </p>
      </div>

      <Input
        placeholder="Search persona packs…"
        value={search}
        onChange={(e) => setSearch(e.target.value)}
        className="max-w-sm"
      />

      {isLoading ? (
        <div className="space-y-2">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-12 w-full" />
          ))}
        </div>
      ) : filtered.length === 0 ? (
        <p className="py-8 text-center text-muted-foreground">
          No persona packs found
        </p>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Category</TableHead>
              <TableHead>Version</TableHead>
              <TableHead>Personas</TableHead>
              <TableHead>Installed</TableHead>
              <TableHead>Phase</TableHead>
              <TableHead>Enabled</TableHead>
              <TableHead>Age</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {filtered.map((pack) => (
              <TableRow key={pack.metadata.name}>
                <TableCell className="font-mono text-sm">
                  <Link
                    to={`/personas/${pack.metadata.name}`}
                    className="hover:text-primary flex items-center gap-1"
                  >
                    {pack.metadata.name}
                    <ExternalLink className="h-3 w-3 opacity-50" />
                  </Link>
                </TableCell>
                <TableCell>
                  {pack.spec.category ? (
                    <Badge variant="outline" className="text-xs capitalize">
                      {pack.spec.category}
                    </Badge>
                  ) : (
                    "—"
                  )}
                </TableCell>
                <TableCell className="text-sm text-muted-foreground">
                  {pack.spec.version || "—"}
                </TableCell>
                <TableCell className="text-sm">
                  {pack.status?.personaCount ?? pack.spec.personas?.length ?? 0}
                </TableCell>
                <TableCell className="text-sm">
                  {pack.status?.installedCount ?? 0}
                </TableCell>
                <TableCell>
                  <StatusBadge phase={pack.status?.phase} />
                </TableCell>
                <TableCell>
                  {pack.spec.enabled ? (
                    <Badge variant="default" className="text-xs">Yes</Badge>
                  ) : (
                    <Badge variant="secondary" className="text-xs">No</Badge>
                  )}
                </TableCell>
                <TableCell className="text-sm text-muted-foreground">
                  {formatAge(pack.metadata.creationTimestamp)}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </div>
  );
}
