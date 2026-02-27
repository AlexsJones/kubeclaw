import { Badge } from "@/components/ui/badge";
import { phaseColor } from "@/lib/utils";

interface StatusBadgeProps {
  phase: string | undefined;
}

export function StatusBadge({ phase }: StatusBadgeProps) {
  if (!phase) return <Badge variant="secondary">Unknown</Badge>;
  return (
    <Badge variant="outline" className={phaseColor(phase)}>
      {phase}
    </Badge>
  );
}
