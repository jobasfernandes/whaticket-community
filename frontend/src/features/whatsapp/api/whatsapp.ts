import api from "@/lib/api";
import type { Queue, Whatsapp } from "@/types/domain";

export interface WhatsappPayload {
  name: string;
  greetingMessage?: string;
  farewellMessage?: string;
  isDefault: boolean;
  queueIds: number[];
}

export async function listWhatsapps(): Promise<Whatsapp[]> {
  const { data } = await api.get<Whatsapp[]>("/whatsapp/");
  return data;
}

export async function getWhatsapp(id: number): Promise<Whatsapp & { queues?: Queue[] }> {
  const { data } = await api.get<Whatsapp & { queues?: Queue[] }>(`/whatsapp/${id}`);
  return data;
}

export async function createWhatsapp(payload: WhatsappPayload): Promise<Whatsapp> {
  const { data } = await api.post<Whatsapp>("/whatsapp", payload);
  return data;
}

export async function updateWhatsapp(id: number, payload: WhatsappPayload): Promise<Whatsapp> {
  const { data } = await api.put<Whatsapp>(`/whatsapp/${id}`, payload);
  return data;
}

export async function deleteWhatsapp(id: number): Promise<void> {
  await api.delete(`/whatsapp/${id}`);
}

export async function startSession(id: number): Promise<void> {
  await api.post(`/whatsappsession/${id}`);
}

export async function requestNewQrCode(id: number): Promise<void> {
  await api.put(`/whatsappsession/${id}`);
}

export async function disconnectSession(id: number): Promise<void> {
  await api.delete(`/whatsappsession/${id}`);
}
