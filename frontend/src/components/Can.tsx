import type { ReactNode } from "react";
import rules, { type Role } from "@/utils/rules";

interface CanProps {
  role?: Role;
  perform: string;
  data?: Record<string, unknown>;
  yes?: () => ReactNode;
  no?: () => ReactNode;
}

const check = (role: Role | undefined, action: string): boolean => {
  if (!role) return false;
  const permissions = rules[role];
  if (!permissions) return false;
  return permissions.static.includes(action);
};

export const Can = ({ role, perform, yes, no }: CanProps) => {
  const allowed = check(role, perform);
  if (allowed && yes) return <>{yes()}</>;
  if (!allowed && no) return <>{no()}</>;
  return null;
};

export default Can;
