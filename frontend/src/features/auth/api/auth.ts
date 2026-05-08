import api from "@/lib/api";
import type { AuthLoginPayload, AuthLoginResponse } from "@/types/domain";

export async function login(payload: AuthLoginPayload): Promise<AuthLoginResponse> {
  const { data } = await api.post<AuthLoginResponse>("/auth/login", payload);
  return data;
}

export async function refresh(): Promise<AuthLoginResponse> {
  const { data } = await api.post<AuthLoginResponse>("/auth/refresh_token");
  return data;
}

export async function logout(): Promise<void> {
  await api.delete("/auth/logout");
}

export interface SignupPayload {
  name: string;
  email: string;
  password: string;
}

export async function signup(payload: SignupPayload): Promise<void> {
  await api.post("/auth/signup", payload);
}
