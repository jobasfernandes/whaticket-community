import { createContext, useContext, type ReactNode } from "react";
import useWhatsApps from "@/features/whatsapp/hooks/useWhatsApps";
import type { Whatsapp } from "@/types/domain";

interface WhatsAppsContextValue {
  loading: boolean;
  whatsApps: Whatsapp[];
}

const WhatsAppsContext = createContext<WhatsAppsContextValue | null>(null);

export const WhatsAppsProvider = ({ children }: { children: ReactNode }) => {
  const value = useWhatsApps();
  return <WhatsAppsContext.Provider value={value}>{children}</WhatsAppsContext.Provider>;
};

export const useWhatsAppsContext = (): WhatsAppsContextValue => {
  const ctx = useContext(WhatsAppsContext);
  if (!ctx) throw new Error("useWhatsAppsContext must be used within WhatsAppsProvider");
  return ctx;
};
