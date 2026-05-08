import { useEffect, useReducer, useState } from "react";
import openSocket from "@/lib/socket";
import toastError from "@/utils/toastError";
import { listWhatsapps } from "@/features/whatsapp/api/whatsapp";
import type { Whatsapp } from "@/types/domain";

type Action =
  | { type: "LOAD_WHATSAPPS"; payload: Whatsapp[] }
  | { type: "UPDATE_WHATSAPPS"; payload: Whatsapp }
  | { type: "UPDATE_SESSION"; payload: Whatsapp }
  | { type: "DELETE_WHATSAPPS"; payload: number }
  | { type: "RESET" };

const reducer = (state: Whatsapp[], action: Action): Whatsapp[] => {
  switch (action.type) {
    case "LOAD_WHATSAPPS":
      return [...action.payload];
    case "UPDATE_WHATSAPPS": {
      const whatsApp = action.payload;
      const idx = state.findIndex((s) => s.id === whatsApp.id);
      if (idx !== -1) {
        const next = [...state];
        next[idx] = whatsApp;
        return next;
      }
      return [whatsApp, ...state];
    }
    case "UPDATE_SESSION": {
      const whatsApp = action.payload;
      const idx = state.findIndex((s) => s.id === whatsApp.id);
      if (idx === -1) return state;
      const next = [...state];
      next[idx] = {
        ...next[idx],
        status: whatsApp.status,
        updatedAt: whatsApp.updatedAt,
        qrcode: whatsApp.qrcode,
        retries: whatsApp.retries,
      };
      return next;
    }
    case "DELETE_WHATSAPPS":
      return state.filter((s) => s.id !== action.payload);
    case "RESET":
      return [];
    default:
      return state;
  }
};

const useWhatsApps = () => {
  const [whatsApps, dispatch] = useReducer(reducer, []);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    setLoading(true);
    (async () => {
      try {
        const data = await listWhatsapps();
        dispatch({ type: "LOAD_WHATSAPPS", payload: data });
      } catch (err) {
        toastError(err);
      } finally {
        setLoading(false);
      }
    })();
  }, []);

  useEffect(() => {
    const socket = openSocket();

    socket.on("whatsapp", (data: unknown) => {
      const evt = data as { action?: string; whatsapp?: Whatsapp; whatsappId?: number };
      if (evt?.action === "update" && evt.whatsapp) {
        dispatch({ type: "UPDATE_WHATSAPPS", payload: evt.whatsapp });
      } else if (evt?.action === "delete" && typeof evt.whatsappId === "number") {
        dispatch({ type: "DELETE_WHATSAPPS", payload: evt.whatsappId });
      }
    });

    socket.on("whatsappSession", (data: unknown) => {
      const evt = data as { action?: string; session?: Whatsapp };
      if (evt?.action === "update" && evt.session) {
        dispatch({ type: "UPDATE_SESSION", payload: evt.session });
      }
    });

    return () => {
      socket.disconnect();
    };
  }, []);

  return { whatsApps, loading };
};

export default useWhatsApps;
