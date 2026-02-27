import { useAuth } from "@/components/auth-provider";
import { getNamespace, setNamespace } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { LogOut, Wifi, WifiOff } from "lucide-react";
import { useWebSocket } from "@/hooks/use-websocket";
import { useState } from "react";

export function Header() {
  const { logout } = useAuth();
  const { connected } = useWebSocket();
  const [ns, setNs] = useState(getNamespace());

  const handleNsChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setNs(e.target.value);
  };

  const handleNsBlur = () => {
    if (ns.trim()) {
      setNamespace(ns.trim());
      window.location.reload();
    }
  };

  const handleNsKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter") handleNsBlur();
  };

  return (
    <header className="flex h-14 items-center justify-between border-b bg-card px-6">
      <div className="flex items-center gap-4">
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <span>Namespace:</span>
          <Input
            value={ns}
            onChange={handleNsChange}
            onBlur={handleNsBlur}
            onKeyDown={handleNsKeyDown}
            className="h-7 w-36 text-xs"
          />
        </div>
      </div>
      <div className="flex items-center gap-3">
        <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
          {connected ? (
            <>
              <Wifi className="h-3.5 w-3.5 text-emerald-400" />
              <span className="text-emerald-400">Stream</span>
            </>
          ) : (
            <>
              <WifiOff className="h-3.5 w-3.5" />
              <span>Offline</span>
            </>
          )}
        </div>
        <Button variant="ghost" size="icon" onClick={logout} title="Logout">
          <LogOut className="h-4 w-4" />
        </Button>
      </div>
    </header>
  );
}
