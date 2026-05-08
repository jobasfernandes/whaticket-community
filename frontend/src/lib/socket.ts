import { getBackendUrl } from "@/config/env";
import { onAuthEvent, tokenStore } from "@/lib/api";

const RECONNECT_INITIAL_MS = 1000;
const RECONNECT_MAX_MS = 30000;

export const KNOWN_BUCKETS = [
  "connect",
  "disconnect",
  "error",
  "appMessage",
  "ticket",
  "contact",
  "whatsapp",
  "whatsappSession",
  "user",
  "queue",
  "quickAnswer",
  "settings",
] as const;

export type Bucket = (typeof KNOWN_BUCKETS)[number];

type Listener = (data: unknown) => void;

type ServerMessage = {
  channel?: string;
  event?: string;
  data?: unknown;
};

type ChannelAction = { action: "subscribe" | "unsubscribe"; channel: string };

const readToken = (): string | null => tokenStore.read();

function buildWsUrl(backendUrl: string, token: string): string {
  const base = backendUrl || `${window.location.protocol}//${window.location.host}`;
  const u = new URL("/ws", base);
  if (u.protocol === "https:") u.protocol = "wss:";
  else if (u.protocol === "http:") u.protocol = "ws:";
  u.searchParams.set("token", token);
  return u.toString();
}

function eventToBucket(channel: string | undefined, event: string | undefined): Bucket | null {
  if (!event) return null;
  if (channel === "system" && event === "connected") return "connect";
  if (channel === "system" && event === "closing") return "disconnect";
  if (event === "error") return "error";
  if (event.startsWith("appMessage")) return "appMessage";
  if (event.startsWith("ticket.")) return "ticket";
  if (event.startsWith("contact.")) return "contact";
  if (event.startsWith("whatsappSession.")) return "whatsappSession";
  if (event.startsWith("whatsapp.")) return "whatsapp";
  if (event.startsWith("user.")) return "user";
  if (event.startsWith("queue.")) return "queue";
  if (event.startsWith("quickAnswer.")) return "quickAnswer";
  if (event.startsWith("settings.")) return "settings";
  return null;
}

function emitToMessages(name: string, args: unknown[]): ChannelAction[] {
  switch (name) {
    case "joinChatBox":
      return [{ action: "subscribe", channel: `ticket:${args[0]}` }];
    case "joinNotification":
      return [
        { action: "subscribe", channel: "notification" },
        { action: "subscribe", channel: "global" },
      ];
    case "joinTickets":
      return [{ action: "subscribe", channel: `tickets:${args[0]}` }];
    case "leaveChatBox":
      return [{ action: "unsubscribe", channel: `ticket:${args[0]}` }];
    case "leaveTickets":
      return [{ action: "unsubscribe", channel: `tickets:${args[0]}` }];
    case "leaveNotification":
      return [
        { action: "unsubscribe", channel: "notification" },
        { action: "unsubscribe", channel: "global" },
      ];
    default:
      console.warn(`[socket-io] ignoring unknown emit "${name}"`);
      return [];
  }
}

class SharedClient {
  private listeners = new Map<Bucket, Set<Listener>>();
  private channelRefs = new Map<string, number>();
  private queue: string[] = [];
  private ws: WebSocket | null = null;
  private reconnectMs = RECONNECT_INITIAL_MS;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private token: string | null = null;

  constructor() {
    this.connect();
  }

  connect() {
    const token = readToken();
    if (!token) {
      this.scheduleReconnect();
      return;
    }
    this.token = token;
    let url: string;
    try {
      url = buildWsUrl(getBackendUrl(), token);
    } catch (err) {
      console.error("[socket-io] invalid backend URL", err);
      return;
    }
    let ws: WebSocket;
    try {
      ws = new WebSocket(url);
    } catch (err) {
      console.error("[socket-io] failed to open WebSocket", err);
      this.scheduleReconnect();
      return;
    }
    this.ws = ws;

    ws.addEventListener("open", () => {
      this.reconnectMs = RECONNECT_INITIAL_MS;
      this.channelRefs.forEach((_count, channel) => {
        ws.send(JSON.stringify({ action: "subscribe", channel }));
      });
      const pending = this.queue.splice(0);
      pending.forEach((msg) => ws.send(msg));
    });

    ws.addEventListener("message", (ev) => {
      let msg: ServerMessage | null = null;
      try {
        msg = JSON.parse(ev.data) as ServerMessage;
      } catch {
        return;
      }
      if (!msg || typeof msg !== "object") return;
      const bucket = eventToBucket(msg.channel, msg.event);
      if (bucket === "connect") {
        this.fire("connect", null);
        return;
      }
      if (bucket === "disconnect") {
        this.fire("disconnect", null);
        return;
      }
      if (bucket === "error") {
        console.warn("[socket-io] server error", msg.channel, msg.data);
        this.fire("error", msg.data);
        return;
      }
      if (bucket) {
        this.fire(bucket, msg.data);
      }
    });

    ws.addEventListener("close", () => {
      this.fire("disconnect", null);
      this.ws = null;
      this.scheduleReconnect();
    });

    ws.addEventListener("error", () => {});
  }

  private scheduleReconnect() {
    if (this.reconnectTimer) return;
    const delay = this.reconnectMs;
    this.reconnectMs = Math.min(this.reconnectMs * 2, RECONNECT_MAX_MS);
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      this.connect();
    }, delay);
  }

  private fire(bucket: Bucket, data: unknown) {
    const set = this.listeners.get(bucket);
    if (!set) return;
    set.forEach((fn) => {
      try {
        fn(data);
      } catch (err) {
        console.error(`[socket-io] listener for "${bucket}" threw`, err);
      }
    });
  }

  addListener(bucket: Bucket, fn: Listener) {
    if (!this.listeners.has(bucket)) this.listeners.set(bucket, new Set());
    this.listeners.get(bucket)!.add(fn);
  }

  removeListener(bucket: Bucket, fn?: Listener) {
    const set = this.listeners.get(bucket);
    if (!set) return;
    if (fn) set.delete(fn);
    else set.clear();
    if (set.size === 0) this.listeners.delete(bucket);
  }

  acquireChannel(channel: string) {
    const current = this.channelRefs.get(channel) ?? 0;
    this.channelRefs.set(channel, current + 1);
    if (current === 0) {
      this.send({ action: "subscribe", channel });
    }
  }

  releaseChannel(channel: string) {
    const current = this.channelRefs.get(channel) ?? 0;
    if (current <= 1) {
      this.channelRefs.delete(channel);
      this.send({ action: "unsubscribe", channel });
    } else {
      this.channelRefs.set(channel, current - 1);
    }
  }

  private send(message: ChannelAction) {
    const json = JSON.stringify(message);
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(json);
    } else {
      this.queue.push(json);
    }
  }

  refreshToken() {
    const next = readToken();
    if (next === this.token) return;
    if (this.ws) {
      try {
        this.ws.close(1000, "token rotated");
      } catch {
        // ignore
      }
      this.ws = null;
    }
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    this.reconnectMs = RECONNECT_INITIAL_MS;
    this.connect();
  }
}

let sharedInstance: SharedClient | null = null;

function getShared(): SharedClient {
  if (!sharedInstance) {
    sharedInstance = new SharedClient();
  }
  return sharedInstance;
}

onAuthEvent((event: string) => {
  if (!sharedInstance) return;
  if (event === "token-changed" || event === "unauthenticated") {
    sharedInstance.refreshToken();
  }
});

export class SocketHandle {
  private localListeners = new Map<Bucket, Set<Listener>>();
  private acquiredChannels = new Map<string, number>();
  private disposed = false;

  constructor(private shared: SharedClient) {}

  on(bucket: Bucket, fn: Listener): this {
    if (this.disposed) return this;
    if (!this.localListeners.has(bucket)) this.localListeners.set(bucket, new Set());
    this.localListeners.get(bucket)!.add(fn);
    this.shared.addListener(bucket, fn);
    return this;
  }

  off(bucket: Bucket, fn?: Listener): this {
    const set = this.localListeners.get(bucket);
    if (!set) return this;
    if (fn) {
      set.delete(fn);
      this.shared.removeListener(bucket, fn);
    } else {
      set.forEach((existing) => this.shared.removeListener(bucket, existing));
      set.clear();
    }
    if (set.size === 0) this.localListeners.delete(bucket);
    return this;
  }

  removeListener(bucket: Bucket, fn?: Listener): this {
    return this.off(bucket, fn);
  }

  removeAllListeners(bucket?: Bucket): this {
    if (bucket) {
      this.off(bucket);
    } else {
      this.localListeners.forEach((set, key) => {
        set.forEach((fn) => this.shared.removeListener(key, fn));
      });
      this.localListeners.clear();
    }
    return this;
  }

  emit(name: string, ...args: unknown[]): this {
    if (this.disposed) return this;
    const messages = emitToMessages(name, args);
    messages.forEach((m) => {
      if (m.action === "subscribe") {
        const count = this.acquiredChannels.get(m.channel) ?? 0;
        this.acquiredChannels.set(m.channel, count + 1);
        this.shared.acquireChannel(m.channel);
      } else if (m.action === "unsubscribe") {
        const count = this.acquiredChannels.get(m.channel) ?? 0;
        if (count <= 0) return;
        if (count === 1) this.acquiredChannels.delete(m.channel);
        else this.acquiredChannels.set(m.channel, count - 1);
        this.shared.releaseChannel(m.channel);
      }
    });
    return this;
  }

  disconnect(): void {
    if (this.disposed) return;
    this.disposed = true;
    this.localListeners.forEach((set, bucket) => {
      set.forEach((fn) => this.shared.removeListener(bucket, fn));
    });
    this.localListeners.clear();
    this.acquiredChannels.forEach((count, channel) => {
      for (let i = 0; i < count; i += 1) {
        this.shared.releaseChannel(channel);
      }
    });
    this.acquiredChannels.clear();
  }

  close(): void {
    this.disconnect();
  }
}

export default function openSocket(): SocketHandle {
  return new SocketHandle(getShared());
}
