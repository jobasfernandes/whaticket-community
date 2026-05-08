import { useEffect, useState, type ReactNode } from "react";
import { NavLink } from "react-router-dom";
import {
  ChevronLeft,
  ChevronRight,
  Contact2,
  LayoutDashboard,
  MessageCircle,
  MessagesSquare,
  Settings,
  SlidersHorizontal,
  Users,
  Workflow,
} from "lucide-react";
import { i18n } from "@/lib/i18n";
import { useWhatsAppsContext } from "@/features/whatsapp/context/WhatsAppsContext";
import { useAuthContext } from "@/features/auth/context/AuthContext";
import { Can } from "@/components/Can";
import { Badge } from "@/components/ui/badge";
import { Separator } from "@/components/ui/separator";
import { cn } from "@/lib/utils";

interface SidebarProps {
  collapsed: boolean;
  onToggle: () => void;
}

interface NavEntry {
  to: string;
  label: string;
  icon: ReactNode;
  end?: boolean;
  badge?: ReactNode;
}

const Sidebar = ({ collapsed, onToggle }: SidebarProps) => {
  const { whatsApps } = useWhatsAppsContext();
  const { user } = useAuthContext();
  const [connectionWarning, setConnectionWarning] = useState(false);

  useEffect(() => {
    const id = setTimeout(() => {
      if (whatsApps.length === 0) {
        setConnectionWarning(false);
        return;
      }
      const offline = whatsApps.some((w) =>
        ["qrcode", "PAIRING", "DISCONNECTED", "TIMEOUT", "OPENING"].includes(w.status),
      );
      setConnectionWarning(offline);
    }, 2000);
    return () => clearTimeout(id);
  }, [whatsApps]);

  const baseEntries: NavEntry[] = [
    { to: "/", label: "Dashboard", icon: <LayoutDashboard className="size-4" />, end: true },
    {
      to: "/connections",
      label: i18n.t("mainDrawer.listItems.connections"),
      icon: <SlidersHorizontal className="size-4" />,
      badge: connectionWarning ? (
        <Badge variant="destructive" className="ml-auto px-1.5 py-0 text-[10px]">
          !
        </Badge>
      ) : null,
    },
    {
      to: "/tickets",
      label: i18n.t("mainDrawer.listItems.tickets"),
      icon: <MessageCircle className="size-4" />,
    },
    {
      to: "/contacts",
      label: i18n.t("mainDrawer.listItems.contacts"),
      icon: <Contact2 className="size-4" />,
    },
    {
      to: "/quickAnswers",
      label: i18n.t("mainDrawer.listItems.quickAnswers"),
      icon: <MessagesSquare className="size-4" />,
    },
  ];

  const adminEntries: NavEntry[] = [
    { to: "/users", label: i18n.t("mainDrawer.listItems.users"), icon: <Users className="size-4" /> },
    { to: "/queues", label: i18n.t("mainDrawer.listItems.queues"), icon: <Workflow className="size-4" /> },
    { to: "/settings", label: i18n.t("mainDrawer.listItems.settings"), icon: <Settings className="size-4" /> },
  ];

  return (
    <aside
      className={cn(
        "flex h-full flex-col border-r bg-sidebar text-sidebar-foreground transition-[width] duration-200",
        collapsed ? "w-16" : "w-60",
      )}
    >
      <div className="flex h-12 items-center justify-between px-3">
        {!collapsed && <span className="text-sm font-semibold">WhaTicket</span>}
        <button
          type="button"
          onClick={onToggle}
          className="ml-auto rounded p-1 text-sidebar-foreground/70 hover:bg-sidebar-accent hover:text-sidebar-foreground"
          aria-label="Toggle sidebar"
        >
          {collapsed ? <ChevronRight className="size-4" /> : <ChevronLeft className="size-4" />}
        </button>
      </div>
      <Separator />
      <nav className="flex-1 space-y-1 overflow-y-auto p-2">
        {baseEntries.map((e) => (
          <SidebarLink key={e.to} entry={e} collapsed={collapsed} />
        ))}
        <Can
          role={user?.profile}
          perform="drawer-admin-items:view"
          yes={() => (
            <>
              <Separator className="my-2" />
              {!collapsed && (
                <p className="px-2 pb-1 text-xs font-medium uppercase text-sidebar-foreground/60">
                  {i18n.t("mainDrawer.listItems.administration")}
                </p>
              )}
              {adminEntries.map((e) => (
                <SidebarLink key={e.to} entry={e} collapsed={collapsed} />
              ))}
            </>
          )}
        />
      </nav>
    </aside>
  );
};

const SidebarLink = ({ entry, collapsed }: { entry: NavEntry; collapsed: boolean }) => (
  <NavLink
    to={entry.to}
    end={entry.end}
    className={({ isActive }) =>
      cn(
        "flex items-center gap-3 rounded-md px-2 py-2 text-sm transition-colors",
        "hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
        isActive && "bg-sidebar-primary text-sidebar-primary-foreground",
        collapsed && "justify-center",
      )
    }
  >
    {entry.icon}
    {!collapsed && <span className="truncate">{entry.label}</span>}
    {!collapsed && entry.badge}
  </NavLink>
);

export default Sidebar;
