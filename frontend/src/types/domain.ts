export type Role = "user" | "admin";

export interface User {
  id: number;
  name: string;
  email: string;
  profile: Role;
  createdAt: string;
  updatedAt: string;
  queues?: Queue[];
}

export interface Queue {
  id: number;
  name: string;
  color: string;
  greetingMessage?: string | null;
}

export interface Contact {
  id: number;
  name: string;
  number: string;
  email?: string | null;
  profilePicUrl?: string | null;
  isGroup?: boolean;
  extraInfo?: ContactExtraInfo[];
  createdAt: string;
  updatedAt: string;
}

export interface ContactExtraInfo {
  id?: number;
  name: string;
  value: string;
}

export interface Whatsapp {
  id: number;
  name: string;
  status: string;
  qrcode?: string | null;
  retries: number;
  isDefault: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface Ticket {
  id: number;
  uuid?: string;
  status: "open" | "pending" | "closed";
  unreadMessages: number;
  lastMessage: string;
  isGroup: boolean;
  userId: number | null;
  contactId: number;
  whatsappId: number;
  queueId: number | null;
  contact: Contact;
  user?: User | null;
  queue?: Queue | null;
  whatsapp?: Whatsapp | null;
  createdAt: string;
  updatedAt: string;
}

export interface Message {
  id: string;
  body: string;
  ack: number;
  read: boolean;
  mediaType: string;
  mediaUrl?: string | null;
  fromMe: boolean;
  isDeleted: boolean;
  ticketId: number;
  contactId?: number;
  contact?: Contact;
  quotedMsg?: Message | null;
  quotedMsgId?: string | null;
  createdAt: string;
  updatedAt: string;
}

export interface QuickAnswer {
  id: number;
  shortcut: string;
  message: string;
  createdAt: string;
  updatedAt: string;
}

export interface AuthLoginPayload {
  email: string;
  password: string;
}

export interface AuthLoginResponse {
  user: User;
  token: string;
}
