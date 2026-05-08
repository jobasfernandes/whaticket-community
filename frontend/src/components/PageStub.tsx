import { Construction } from "lucide-react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

interface Props {
  title: string;
  description?: string;
}

const PageStub = ({ title, description }: Props) => {
  return (
    <Card className="mx-auto mt-8 max-w-2xl">
      <CardHeader>
        <div className="flex items-center gap-3">
          <Construction className="size-6 text-primary" />
          <div>
            <CardTitle>{title}</CardTitle>
            <CardDescription>
              {description ?? "Esta tela ainda não foi reconstruída na nova stack (shadcn + Tailwind)."}
            </CardDescription>
          </div>
        </div>
      </CardHeader>
      <CardContent className="text-sm text-muted-foreground">
        A migração big-bang para React 19 + shadcn/Radix + Tailwind v4 foi feita em fases.
        Foundation, autenticação, sidebar e navegação estão prontas. As telas de domínio
        (chat, listas, modais) estão sendo reconstruídas.
      </CardContent>
    </Card>
  );
};

export default PageStub;
