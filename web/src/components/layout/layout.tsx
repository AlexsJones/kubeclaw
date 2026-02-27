import { Outlet } from "react-router-dom";
import { AppSidebar } from "./sidebar";
import { Header } from "./header";

export function Layout() {
  return (
    <div className="flex h-screen overflow-hidden">
      <AppSidebar />
      <div className="flex flex-1 flex-col overflow-hidden">
        <Header />
        <main className="flex-1 overflow-auto p-6">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
