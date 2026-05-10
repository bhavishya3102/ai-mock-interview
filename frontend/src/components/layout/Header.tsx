import type { ReactElement } from "react";
import { Link, NavLink } from "react-router-dom";
import { SignedIn, SignedOut, UserButton } from "@clerk/clerk-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/cn";

const navItems = [
  { to: "/dashboard", label: "Dashboard" },
  { to: "/", label: "Home" },
] as const;

export function Header(): ReactElement {
  return (
    <header className="sticky top-0 z-40 w-full border-b border-border/60 bg-background/80 backdrop-blur supports-[backdrop-filter]:bg-background/60">
      <div className="mx-auto flex h-16 w-full max-w-6xl items-center justify-between gap-6 px-6">
        <Link to="/" className="flex items-center gap-2">
          <img src="/logo.png" alt="" className="h-8 w-8 object-contain" />
          <span className="font-display text-xl font-semibold tracking-tight">Eunoia</span>
        </Link>
        <SignedIn>
          <nav className="hidden items-center gap-1 md:flex">
            {navItems.map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                end={item.to === "/"}
                className={({ isActive }) =>
                  cn(
                    "rounded-md px-3 py-2 text-sm font-medium text-muted-foreground transition-colors hover:text-foreground",
                    isActive && "text-foreground",
                  )
                }
              >
                {item.label}
              </NavLink>
            ))}
          </nav>
        </SignedIn>
        <div className="flex items-center gap-3">
          <SignedOut>
            <Button asChild variant="ghost" size="sm">
              <Link to="/sign-in">Sign in</Link>
            </Button>
            <Button asChild size="sm">
              <Link to="/sign-up">Get started</Link>
            </Button>
          </SignedOut>
          <SignedIn>
            <UserButton afterSignOutUrl="/" />
          </SignedIn>
        </div>
      </div>
    </header>
  );
}
