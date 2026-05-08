import { useState } from "react";
import { Outlet } from "react-router-dom";
import { LogOut, User as UserIcon } from "lucide-react";

import { useAuthContext } from "@/features/auth/context/AuthContext";
import { ReplyMessageProvider } from "@/features/messages/context/ReplyingMessageContext";
import BackdropLoading from "@/components/BackdropLoading";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { i18n } from "@/lib/i18n";
import AppSidebar from "./AppSidebar";
import ThemeToggle from "./ThemeToggle";

const initials = (name: string | undefined): string => {
  if (!name) return "?";
  return name
    .split(/\s+/)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase() ?? "")
    .join("");
};

const LoggedInLayout = () => {
  const { user, loading, handleLogout } = useAuthContext();
  const [collapsed, setCollapsed] = useState(false);

  if (loading) return <BackdropLoading />;

  return (
    <ReplyMessageProvider>
      <div className="flex h-screen overflow-hidden bg-background">
        <AppSidebar collapsed={collapsed} onToggle={() => setCollapsed((v) => !v)} />
        <div className="flex flex-1 flex-col overflow-hidden">
          <header className="flex h-14 items-center justify-end gap-2 border-b bg-background px-4">
            <ThemeToggle />
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" size="icon" aria-label="User menu">
                  <Avatar className="size-8">
                    <AvatarFallback>{initials(user?.name)}</AvatarFallback>
                  </Avatar>
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="w-48">
                <DropdownMenuLabel className="truncate">{user?.name ?? "—"}</DropdownMenuLabel>
                <DropdownMenuSeparator />
                <DropdownMenuItem disabled>
                  <UserIcon className="size-4" />
                  {i18n.t("mainDrawer.appBar.user.profile")}
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => void handleLogout()}>
                  <LogOut className="size-4" />
                  {i18n.t("mainDrawer.appBar.user.logout")}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </header>
          <main className="flex-1 overflow-auto p-4">
            <Outlet />
          </main>
        </div>
      </div>
    </ReplyMessageProvider>
  );
};

export default LoggedInLayout;
