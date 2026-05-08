import type { ReactNode } from "react";

interface Props {
  title: string;
  actions?: ReactNode;
}

const PageHeader = ({ title, actions }: Props) => {
  return (
    <header className="mb-4 flex items-center justify-between gap-4">
      <h1 className="text-xl font-semibold tracking-tight">{title}</h1>
      {actions && <div className="flex items-center gap-2">{actions}</div>}
    </header>
  );
};

export default PageHeader;
