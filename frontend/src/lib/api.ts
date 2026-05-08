import axios, {
  type AxiosInstance,
  type InternalAxiosRequestConfig,
} from "axios";
import { getBackendUrl } from "@/config/env";

type RetriableConfig = InternalAxiosRequestConfig & { _retry?: boolean };

const TOKEN_KEY = "token";

export const tokenStore = {
  read(): string | null {
    const raw = localStorage.getItem(TOKEN_KEY);
    if (!raw) return null;
    try {
      return JSON.parse(raw) as string;
    } catch {
      return raw;
    }
  },
  write(token: string): void {
    localStorage.setItem(TOKEN_KEY, JSON.stringify(token));
  },
  clear(): void {
    localStorage.removeItem(TOKEN_KEY);
  },
};

type AuthEvent = "token-changed" | "unauthenticated";
type AuthListener = (event: AuthEvent) => void;

const authListeners = new Set<AuthListener>();

export function onAuthEvent(listener: AuthListener): () => void {
  authListeners.add(listener);
  return () => {
    authListeners.delete(listener);
  };
}

function emitAuthEvent(event: AuthEvent): void {
  authListeners.forEach((fn) => {
    try {
      fn(event);
    } catch (err) {
      console.error("[auth] listener threw", err);
    }
  });
}

const api: AxiosInstance = axios.create({
  baseURL: getBackendUrl(),
  withCredentials: true,
  timeout: 15000,
});

api.interceptors.request.use((config) => {
  const token = tokenStore.read();
  if (token) config.headers.set("Authorization", `Bearer ${token}`);
  return config;
});

api.interceptors.response.use(
  (response) => response,
  async (error: { response?: { status?: number }; config?: RetriableConfig }) => {
    const originalRequest = error.config;
    if (error?.response?.status === 403 && originalRequest && !originalRequest._retry) {
      originalRequest._retry = true;
      try {
        const { data } = await api.post<{ token: string }>("/auth/refresh_token");
        if (data?.token) {
          tokenStore.write(data.token);
          api.defaults.headers.common.Authorization = `Bearer ${data.token}`;
          emitAuthEvent("token-changed");
        }
        return api.request(originalRequest);
      } catch (refreshErr) {
        tokenStore.clear();
        delete api.defaults.headers.common.Authorization;
        emitAuthEvent("unauthenticated");
        return Promise.reject(refreshErr);
      }
    }
    if (error?.response?.status === 401) {
      tokenStore.clear();
      delete api.defaults.headers.common.Authorization;
      emitAuthEvent("unauthenticated");
    }
    return Promise.reject(error);
  },
);

export function setAuthToken(token: string | null): void {
  if (token) {
    tokenStore.write(token);
    api.defaults.headers.common.Authorization = `Bearer ${token}`;
    emitAuthEvent("token-changed");
  } else {
    tokenStore.clear();
    delete api.defaults.headers.common.Authorization;
    emitAuthEvent("unauthenticated");
  }
}

export default api;
