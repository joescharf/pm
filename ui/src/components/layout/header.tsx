import { Moon, Sun } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useEffect, useState } from "react";

export function Header() {
  const [dark, setDark] = useState(() => {
    if (typeof window !== "undefined") {
      const stored = localStorage.getItem("pm-dark-mode");
      if (stored !== null) return stored === "true";
      return document.documentElement.classList.contains("dark");
    }
    return false;
  });

  useEffect(() => {
    document.documentElement.classList.toggle("dark", dark);
    localStorage.setItem("pm-dark-mode", String(dark));
  }, [dark]);

  return (
    <header className="h-12 border-b flex items-center justify-between px-4">
      <div />
      <Button
        variant="ghost"
        size="icon-sm"
        onClick={() => setDark(!dark)}
        aria-label="Toggle dark mode"
      >
        {dark ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
      </Button>
    </header>
  );
}
