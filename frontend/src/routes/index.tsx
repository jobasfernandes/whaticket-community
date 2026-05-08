import { lazy, Suspense } from "react";
import { BrowserRouter, Navigate, Route, Routes as RouterRoutes } from "react-router-dom";

import { AuthProvider } from "@/features/auth/context/AuthContext";
import PrivateRoute from "@/features/auth/routes/PrivateRoute";
import PublicRoute from "@/features/auth/routes/PublicRoute";
import { WhatsAppsProvider } from "@/features/whatsapp/context/WhatsAppsContext";
import AppLayout from "@/layouts/AppLayout";
import BackdropLoading from "@/components/BackdropLoading";

const LoginPage = lazy(() => import("@/pages/LoginPage"));
const SignupPage = lazy(() => import("@/pages/SignupPage"));
const DashboardPage = lazy(() => import("@/pages/DashboardPage"));
const TicketsPage = lazy(() => import("@/pages/TicketsPage"));
const ConnectionsPage = lazy(() => import("@/pages/ConnectionsPage"));
const ContactsPage = lazy(() => import("@/pages/ContactsPage"));
const UsersPage = lazy(() => import("@/pages/UsersPage"));
const QuickAnswersPage = lazy(() => import("@/pages/QuickAnswersPage"));
const QueuesPage = lazy(() => import("@/pages/QueuesPage"));
const SettingsPage = lazy(() => import("@/pages/SettingsPage"));

const Routes = () => {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Suspense fallback={<BackdropLoading />}>
          <RouterRoutes>
            <Route
              path="/login"
              element={
                <PublicRoute>
                  <LoginPage />
                </PublicRoute>
              }
            />
            <Route
              path="/signup"
              element={
                <PublicRoute>
                  <SignupPage />
                </PublicRoute>
              }
            />
            <Route
              element={
                <PrivateRoute>
                  <WhatsAppsProvider>
                    <AppLayout />
                  </WhatsAppsProvider>
                </PrivateRoute>
              }
            >
              <Route path="/" element={<DashboardPage />} />
              <Route path="/tickets" element={<TicketsPage />} />
              <Route path="/tickets/:ticketId" element={<TicketsPage />} />
              <Route path="/connections" element={<ConnectionsPage />} />
              <Route path="/contacts" element={<ContactsPage />} />
              <Route path="/users" element={<UsersPage />} />
              <Route path="/quickAnswers" element={<QuickAnswersPage />} />
              <Route path="/queues" element={<QueuesPage />} />
              <Route path="/settings" element={<SettingsPage />} />
            </Route>
            <Route path="*" element={<Navigate to="/" replace />} />
          </RouterRoutes>
        </Suspense>
      </AuthProvider>
    </BrowserRouter>
  );
};

export default Routes;
