import {
  LayoutDashboard,
  FolderGit2,
  CircleDot,
  Bot,
} from "lucide-react";
import { NavLink } from "react-router";
import { cn } from "@/lib/utils";

const navItems = [
  { to: "/", label: "Dashboard", icon: LayoutDashboard },
  { to: "/projects", label: "Projects", icon: FolderGit2 },
  { to: "/issues", label: "Issues", icon: CircleDot },
  { to: "/sessions", label: "Sessions", icon: Bot },
];

export function Sidebar() {
  return (
    <aside className="w-56 border-r bg-muted/40 flex flex-col shrink-0">
      <div className="p-4 border-b">
        <h1 className="text-lg font-bold tracking-tight">PM</h1>
        <p className="text-xs text-muted-foreground">Project Manager</p>
      </div>
      <nav className="flex-1 p-2 space-y-1">
        {navItems.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            end={item.to === "/"}
            className={({ isActive }) =>
              cn(
                "flex items-center gap-3 px-3 py-2 rounded-md text-sm font-medium transition-colors",
                isActive
                  ? "bg-primary text-primary-foreground"
                  : "text-muted-foreground hover:bg-accent hover:text-accent-foreground",
              )
            }
          >
            <item.icon className="h-4 w-4" />
            {item.label}
          </NavLink>
        ))}
      </nav>
    </aside>
  );
}
