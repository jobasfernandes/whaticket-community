import { createContext, useContext, useState, type ReactNode } from "react";
import type { Message } from "@/types/domain";

interface ReplyMessageContextValue {
  replyingMessage: Message | null;
  setReplyingMessage: (msg: Message | null) => void;
}

const ReplyMessageContext = createContext<ReplyMessageContextValue | null>(null);

export const ReplyMessageProvider = ({ children }: { children: ReactNode }) => {
  const [replyingMessage, setReplyingMessage] = useState<Message | null>(null);
  return (
    <ReplyMessageContext.Provider value={{ replyingMessage, setReplyingMessage }}>
      {children}
    </ReplyMessageContext.Provider>
  );
};

export const useReplyMessageContext = (): ReplyMessageContextValue => {
  const ctx = useContext(ReplyMessageContext);
  if (!ctx) throw new Error("useReplyMessageContext must be used within ReplyMessageProvider");
  return ctx;
};
