import api from "@/lib/api";
import type { Ticket } from "@/types/domain";

export interface ListTicketsParams {
  searchParam?: string;
  pageNumber?: number;
  status?: string;
  date?: string;
  showAll?: boolean;
  queueIds?: number[];
  withUnreadMessages?: boolean;
}

export interface ListTicketsResponse {
  tickets: Ticket[];
  hasMore: boolean;
  count: number;
}

export async function listTickets(params: ListTicketsParams): Promise<ListTicketsResponse> {
  const { data } = await api.get<ListTicketsResponse>("/tickets", { params });
  return data;
}

export async function updateTicket(id: number, payload: Partial<Ticket>): Promise<Ticket> {
  const { data } = await api.put<Ticket>(`/tickets/${id}`, payload);
  return data;
}
