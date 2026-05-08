import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { toast } from "sonner";

import openSocket from "@/lib/socket";
import { setAuthToken, tokenStore } from "@/lib/api";
import { login as apiLogin, logout as apiLogout, refresh as apiRefresh } from "@/features/auth/api/auth";
import toastError from "@/utils/toastError";
import { i18n } from "@/lib/i18n";
import type { AuthLoginPayload, User } from "@/types/domain";

const useAuth = () => {
  const navigate = useNavigate();
  const [isAuth, setIsAuth] = useState(false);
  const [loading, setLoading] = useState(true);
  const [user, setUser] = useState<User | null>(null);

  useEffect(() => {
    const token = tokenStore.read();
    (async () => {
      if (token) {
        try {
          const data = await apiRefresh();
          setAuthToken(data.token);
          setIsAuth(true);
          setUser(data.user);
        } catch (err) {
          toastError(err);
        }
      }
      setLoading(false);
    })();
  }, []);

  const userId = user?.id;

  useEffect(() => {
    if (!userId) return;
    const socket = openSocket();
    socket.on("user", (raw: unknown) => {
      const evt = raw as { action?: string; user?: User };
      if (evt?.action === "update" && evt.user && evt.user.id === userId) {
        setUser(evt.user);
      }
    });
    return () => {
      socket.disconnect();
    };
  }, [userId]);

  const handleLogin = async (userData: AuthLoginPayload) => {
    setLoading(true);
    try {
      const data = await apiLogin(userData);
      setAuthToken(data.token);
      setUser(data.user);
      setIsAuth(true);
      toast.success(i18n.t("auth.toasts.success"));
      navigate("/tickets");
    } catch (err) {
      toastError(err);
    } finally {
      setLoading(false);
    }
  };

  const handleLogout = async () => {
    setLoading(true);
    try {
      await apiLogout();
      setIsAuth(false);
      setUser(null);
      setAuthToken(null);
      navigate("/login");
    } catch (err) {
      toastError(err);
    } finally {
      setLoading(false);
    }
  };

  return { isAuth, user, loading, handleLogin, handleLogout };
};

export default useAuth;
