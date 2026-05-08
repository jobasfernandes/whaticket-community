import api from "@/lib/api";
import type { Queue } from "@/types/domain";

export async function listQueues(): Promise<Queue[]> {
  const { data } = await api.get<Queue[]>("/queue");
  return data;
}
