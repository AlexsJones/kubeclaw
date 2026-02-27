import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/components/auth-provider";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@/components/ui/card";

export function LoginPage() {
  const [token, setToken] = useState("");
  const { login } = useAuth();
  const navigate = useNavigate();

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (token.trim()) {
      login(token.trim());
      navigate("/dashboard");
    }
  };

  return (
    <div className="flex min-h-screen items-center justify-center bg-background">
      <Card className="w-full max-w-md">
        <CardHeader className="text-center">
          <div className="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-xl bg-primary text-primary-foreground font-bold text-2xl">
            S
          </div>
          <CardTitle className="text-2xl">Sympozium</CardTitle>
          <CardDescription>
            Enter your API token to access the dashboard
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="token">API Token</Label>
              <Input
                id="token"
                type="password"
                value={token}
                onChange={(e) => setToken(e.target.value)}
                placeholder="Enter your bearer token"
                autoFocus
              />
            </div>
            <Button type="submit" className="w-full" disabled={!token.trim()}>
              Sign In
            </Button>
            <p className="text-center text-xs text-muted-foreground">
              Provide the token used with{" "}
              <code className="rounded bg-muted px-1 py-0.5">
                sympozium serve --token
              </code>
            </p>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
