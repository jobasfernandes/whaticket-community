function getConfig(name: string, defaultValue: string | null = null): string | null {
  if (typeof window !== "undefined" && window.ENV !== undefined) {
    return window.ENV[name] ?? defaultValue;
  }
  return (import.meta.env as Record<string, string | undefined>)[name] ?? defaultValue;
}

export function getBackendUrl(): string {
  const url = getConfig("VITE_BACKEND_URL");
  if (!url) throw new Error("VITE_BACKEND_URL is not configured");
  return url;
}

export function getHoursCloseTicketsAuto(): string | null {
  return getConfig("VITE_HOURS_CLOSE_TICKETS_AUTO");
}
