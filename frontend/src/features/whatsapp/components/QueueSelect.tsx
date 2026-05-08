import { useEffect, useState } from "react";
import { Check, ChevronsUpDown } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Checkbox } from "@/components/ui/checkbox";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import useQueues from "@/features/queues/hooks/useQueues";
import toastError from "@/utils/toastError";
import { i18n } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import type { Queue } from "@/types/domain";

interface Props {
  selectedQueueIds: number[];
  onChange: (ids: number[]) => void;
}

const QueueSelect = ({ selectedQueueIds, onChange }: Props) => {
  const { findAll } = useQueues();
  const [queues, setQueues] = useState<Queue[]>([]);

  useEffect(() => {
    (async () => {
      try {
        const data = await findAll();
        setQueues(data);
      } catch (err) {
        toastError(err);
      }
    })();
  }, []);

  const toggle = (id: number) => {
    onChange(
      selectedQueueIds.includes(id)
        ? selectedQueueIds.filter((qid) => qid !== id)
        : [...selectedQueueIds, id],
    );
  };

  const selectedQueues = queues.filter((q) => selectedQueueIds.includes(q.id));

  return (
    <div className="space-y-2">
      <Label>{i18n.t("whatsappModal.form.queues") || "Queues"}</Label>
      <Popover>
        <PopoverTrigger asChild>
          <Button variant="outline" className="w-full justify-between">
            <span className="flex flex-wrap gap-1">
              {selectedQueues.length === 0 ? (
                <span className="text-muted-foreground">—</span>
              ) : (
                selectedQueues.map((q) => (
                  <Badge key={q.id} variant="secondary" style={{ backgroundColor: q.color, color: "#fff" }}>
                    {q.name}
                  </Badge>
                ))
              )}
            </span>
            <ChevronsUpDown className="size-4 shrink-0 opacity-50" />
          </Button>
        </PopoverTrigger>
        <PopoverContent className="w-[var(--radix-popover-trigger-width)] p-2">
          {queues.length === 0 ? (
            <p className="px-2 py-3 text-center text-sm text-muted-foreground">No queues</p>
          ) : (
            <ul className="space-y-1">
              {queues.map((queue) => {
                const checked = selectedQueueIds.includes(queue.id);
                return (
                  <li key={queue.id}>
                    <button
                      type="button"
                      onClick={() => toggle(queue.id)}
                      className={cn(
                        "flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-sm hover:bg-accent",
                        checked && "bg-accent",
                      )}
                    >
                      <Checkbox checked={checked} className="pointer-events-none" />
                      <span
                        className="size-3 rounded-full border"
                        style={{ backgroundColor: queue.color }}
                      />
                      <span className="flex-1 text-left">{queue.name}</span>
                      {checked && <Check className="size-4" />}
                    </button>
                  </li>
                );
              })}
            </ul>
          )}
        </PopoverContent>
      </Popover>
    </div>
  );
};

export default QueueSelect;
