import { NavLink } from "react-router-dom";
import {
  LayoutDashboard,
  Server,
  Play,
  Shield,
  Wrench,
  Clock,
  Users,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { ScrollArea } from "@/components/ui/scroll-area";

const navItems = [
  { to: "/dashboard", label: "Dashboard", icon: LayoutDashboard },
  { to: "/instances", label: "Instances", icon: Server },
  { to: "/runs", label: "Runs", icon: Play },
  { to: "/policies", label: "Policies", icon: Shield },
  { to: "/skills", label: "Skills", icon: Wrench },
  { to: "/schedules", label: "Schedules", icon: Clock },
  { to: "/personas", label: "Persona Packs", icon: Users },
];

export function AppSidebar() {
  return (
    <aside className="flex h-full w-60 flex-col border-r bg-card">
      {/* Logo */}
      <div className="flex h-14 items-center gap-2 border-b px-4">
        <div className="flex h-8 w-8 items-center justify-center rounded-md bg-primary text-primary-foreground font-bold text-sm">
          S
        </div>
        <span className="text-base font-semibold tracking-tight">
          Sympozium
        </span>
      </div>

      {/* Navigation */}
      <ScrollArea className="flex-1 py-2">
        <nav className="flex flex-col gap-1 px-2">
          {navItems.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              className={({ isActive }) =>
                cn(
                  "flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors",
                  isActive
                    ? "bg-primary/10 text-primary"
                    : "text-muted-foreground hover:bg-muted hover:text-foreground"
                )
              }
            >
              <item.icon className="h-4 w-4" />
              {item.label}
            </NavLink>
          ))}
        </nav>
      </ScrollArea>

      {/* Footer */}
      <div className="border-t px-4 py-3">
        <p className="text-xs text-muted-foreground">
          Kubernetes-native AI agents
        </p>
      </div>
    </aside>
  );
}
