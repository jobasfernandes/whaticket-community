import { useEffect, useState } from "react";
import { getHoursCloseTicketsAuto } from "@/config/env";
import toastError from "@/utils/toastError";
import { listTickets, updateTicket, type ListTicketsParams } from "@/features/tickets/api/tickets";
import type { Ticket } from "@/types/domain";

const useTickets = ({
  searchParam,
  pageNumber,
  status,
  date,
  showAll,
  queueIds,
  withUnreadMessages,
}: ListTicketsParams) => {
  const [loading, setLoading] = useState(true);
  const [hasMore, setHasMore] = useState(false);
  const [tickets, setTickets] = useState<Ticket[]>([]);
  const [count, setCount] = useState(0);

  useEffect(() => {
    setLoading(true);
    const closeTicket = async (ticket: Ticket) => {
      await updateTicket(ticket.id, { status: "closed", userId: ticket.userId ?? null });
    };

    const delayDebounce = setTimeout(() => {
      (async () => {
        try {
          const data = await listTickets({
            searchParam,
            pageNumber,
            status,
            date,
            showAll,
            queueIds,
            withUnreadMessages,
          });
          setTickets(data.tickets);

          const autoCloseHours = getHoursCloseTicketsAuto();
          if (
            status === "open" &&
            autoCloseHours &&
            autoCloseHours !== "" &&
            autoCloseHours !== "0" &&
            Number(autoCloseHours) > 0
          ) {
            const limit = new Date();
            limit.setHours(limit.getHours() - Number(autoCloseHours));
            data.tickets.forEach((ticket) => {
              if (ticket.status !== "closed") {
                const lastInteraction = new Date(ticket.updatedAt);
                if (lastInteraction < limit) void closeTicket(ticket);
              }
            });
          }

          setHasMore(data.hasMore);
          setCount(data.count);
          setLoading(false);
        } catch (err) {
          setLoading(false);
          toastError(err);
        }
      })();
    }, 500);

    return () => clearTimeout(delayDebounce);
  }, [searchParam, pageNumber, status, date, showAll, queueIds, withUnreadMessages]);

  return { tickets, loading, hasMore, count };
};

export default useTickets;
