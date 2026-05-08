import { createContext, useContext, type ReactNode } from "react";
import useAuth from "@/features/auth/hooks/useAuth";
import type { AuthLoginPayload, User } from "@/types/domain";

interface AuthContextValue {
  loading: boolean;
  isAuth: boolean;
  user: User | null;
  handleLogin: (userData: AuthLoginPayload) => Promise<void>;
  handleLogout: () => Promise<void>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export const AuthProvider = ({ children }: { children: ReactNode }) => {
  const value = useAuth();
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
};

export const useAuthContext = (): AuthContextValue => {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuthContext must be used within AuthProvider");
  return ctx;
};
