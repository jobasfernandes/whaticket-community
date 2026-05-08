import { Navigate, useLocation } from "react-router-dom";
import type { ReactElement } from "react";
import { useAuthContext } from "@/features/auth/context/AuthContext";
import BackdropLoading from "@/components/BackdropLoading";

interface Props {
  children: ReactElement;
}

const PrivateRoute = ({ children }: Props) => {
  const { isAuth, loading } = useAuthContext();
  const location = useLocation();

  if (loading) return <BackdropLoading />;
  if (!isAuth) return <Navigate to="/login" state={{ from: location }} replace />;
  return children;
};

export default PrivateRoute;
