import { listQueues } from "@/features/queues/api/queues";
import type { Queue } from "@/types/domain";

const useQueues = () => {
  const findAll = async (): Promise<Queue[]> => {
    return listQueues();
  };

  return { findAll };
};

export default useQueues;
