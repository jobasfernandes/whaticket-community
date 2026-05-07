import { getBackendUrl } from "../config";

const RECONNECT_INITIAL_MS = 1000;
const RECONNECT_MAX_MS = 30000;

const KNOWN_BUCKETS = [
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
];

function readToken() {
  const raw = localStorage.getItem("token");
  if (!raw) return null;
  try {
    return JSON.parse(raw);
  } catch (_) {
    return raw;
  }
}

function buildWsUrl(backendUrl, token) {
  const base = backendUrl || `${window.location.protocol}//${window.location.host}`;
  const u = new URL("/ws", base);
  if (u.protocol === "https:") u.protocol = "wss:";
  else if (u.protocol === "http:") u.protocol = "ws:";
  u.searchParams.set("token", token);
  return u.toString();
}

function eventToBucket(channel, event) {
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

function emitToMessages(name, args) {
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
      // eslint-disable-next-line no-console
      console.warn(`[socket-io] ignoring unknown emit "${name}"`);
      return [];
  }
}

class WSClient {
  constructor() {
    this.listeners = new Map();
    this.subscribed = new Set();
    this.queue = [];
    this.ws = null;
    this.closed = false;
    this.reconnectMs = RECONNECT_INITIAL_MS;
    this.reconnectTimer = null;
    this.connect();
  }

  connect() {
    if (this.closed) return;
    const token = readToken();
    if (!token) return;
    let url;
    try {
      url = buildWsUrl(getBackendUrl(), token);
    } catch (err) {
      // eslint-disable-next-line no-console
      console.error("[socket-io] invalid backend URL", err);
      return;
    }
    let ws;
    try {
      ws = new WebSocket(url);
    } catch (err) {
      // eslint-disable-next-line no-console
      console.error("[socket-io] failed to open WebSocket", err);
      this.scheduleReconnect();
      return;
    }
    this.ws = ws;

    ws.addEventListener("open", () => {
      this.reconnectMs = RECONNECT_INITIAL_MS;
      this.subscribed.forEach((channel) => {
        ws.send(JSON.stringify({ action: "subscribe", channel }));
      });
      const pending = this.queue.splice(0);
      pending.forEach((msg) => ws.send(msg));
    });

    ws.addEventListener("message", (ev) => {
      let msg;
      try {
        msg = JSON.parse(ev.data);
      } catch (_) {
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
        // eslint-disable-next-line no-console
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

    ws.addEventListener("error", () => {
      // close handler will reconnect
    });
  }

  scheduleReconnect() {
    if (this.closed) return;
    if (this.reconnectTimer) return;
    const delay = this.reconnectMs;
    this.reconnectMs = Math.min(this.reconnectMs * 2, RECONNECT_MAX_MS);
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      this.connect();
    }, delay);
  }

  fire(bucket, data) {
    const set = this.listeners.get(bucket);
    if (!set) return;
    set.forEach((fn) => {
      try {
        fn(data);
      } catch (err) {
        // eslint-disable-next-line no-console
        console.error(`[socket-io] listener for "${bucket}" threw`, err);
      }
    });
  }

  on(bucket, fn) {
    if (typeof fn !== "function") return this;
    if (!this.listeners.has(bucket)) this.listeners.set(bucket, new Set());
    this.listeners.get(bucket).add(fn);
    return this;
  }

  off(bucket, fn) {
    const set = this.listeners.get(bucket);
    if (set) {
      if (fn) set.delete(fn);
      else set.clear();
    }
    return this;
  }

  removeListener(bucket, fn) {
    return this.off(bucket, fn);
  }

  removeAllListeners(bucket) {
    if (bucket) this.listeners.delete(bucket);
    else this.listeners.clear();
    return this;
  }

  emit(name, ...args) {
    const messages = emitToMessages(name, args);
    messages.forEach((m) => {
      const json = JSON.stringify(m);
      if (m.action === "subscribe") this.subscribed.add(m.channel);
      else if (m.action === "unsubscribe") this.subscribed.delete(m.channel);
      if (this.ws && this.ws.readyState === WebSocket.OPEN) {
        this.ws.send(json);
      } else {
        this.queue.push(json);
      }
    });
    return this;
  }

  disconnect() {
    this.closed = true;
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    if (this.ws) {
      try {
        this.ws.close(1000, "client disconnect");
      } catch (_) {
        // ignore
      }
      this.ws = null;
    }
    this.listeners.clear();
    this.subscribed.clear();
    this.queue = [];
  }

  close() {
    return this.disconnect();
  }
}

export { KNOWN_BUCKETS };

export default function openSocket() {
  return new WSClient();
}
