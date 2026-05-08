import { memo, useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { toast } from "sonner";

import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Switch } from "@/components/ui/switch";
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
import toastError from "@/utils/toastError";
import { i18n } from "@/lib/i18n";
import QueueSelect from "@/features/whatsapp/components/QueueSelect";
import { createWhatsapp, getWhatsapp, updateWhatsapp } from "@/features/whatsapp/api/whatsapp";

const whatsappSchema = z.object({
  name: z.string().min(2, "Too Short!").max(50, "Too Long!"),
  greetingMessage: z.string().optional(),
  farewellMessage: z.string().optional(),
  isDefault: z.boolean(),
});

type WhatsAppValues = z.infer<typeof whatsappSchema>;

const initialValues: WhatsAppValues = {
  name: "",
  greetingMessage: "",
  farewellMessage: "",
  isDefault: false,
};

interface Props {
  open: boolean;
  onClose: () => void;
  whatsAppId: number | null | undefined;
}

const WhatsAppModal = ({ open, onClose, whatsAppId }: Props) => {
  const [selectedQueueIds, setSelectedQueueIds] = useState<number[]>([]);

  const form = useForm<WhatsAppValues>({
    resolver: zodResolver(whatsappSchema),
    defaultValues: initialValues,
  });

  useEffect(() => {
    if (!whatsAppId || !open) {
      form.reset(initialValues);
      setSelectedQueueIds([]);
      return;
    }
    (async () => {
      try {
        const data = await getWhatsapp(whatsAppId);
        form.reset({
          name: data.name,
          greetingMessage: (data as { greetingMessage?: string }).greetingMessage ?? "",
          farewellMessage: (data as { farewellMessage?: string }).farewellMessage ?? "",
          isDefault: data.isDefault,
        });
        setSelectedQueueIds(data.queues?.map((q) => q.id) ?? []);
      } catch (err) {
        toastError(err);
      }
    })();
  }, [whatsAppId, open, form]);

  const onSubmit = async (values: WhatsAppValues) => {
    const payload = { ...values, queueIds: selectedQueueIds };
    try {
      if (whatsAppId) {
        await updateWhatsapp(whatsAppId, payload);
      } else {
        await createWhatsapp(payload);
      }
      toast.success(i18n.t("whatsappModal.success"));
      onClose();
    } catch (err) {
      toastError(err);
    }
  };

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>
            {whatsAppId
              ? i18n.t("whatsappModal.title.edit")
              : i18n.t("whatsappModal.title.add")}
          </DialogTitle>
        </DialogHeader>

        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
            <div className="flex items-end gap-4">
              <FormField
                control={form.control}
                name="name"
                render={({ field }) => (
                  <FormItem className="flex-1">
                    <FormLabel>{i18n.t("whatsappModal.form.name")}</FormLabel>
                    <FormControl>
                      <Input autoFocus {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name="isDefault"
                render={({ field }) => (
                  <FormItem className="flex flex-col items-start space-y-2 pb-2">
                    <FormLabel>{i18n.t("whatsappModal.form.default")}</FormLabel>
                    <FormControl>
                      <Switch checked={field.value} onCheckedChange={field.onChange} />
                    </FormControl>
                  </FormItem>
                )}
              />
            </div>

            <FormField
              control={form.control}
              name="greetingMessage"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{i18n.t("queueModal.form.greetingMessage") || "Greeting message"}</FormLabel>
                  <FormControl>
                    <Textarea rows={3} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="farewellMessage"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{i18n.t("whatsappModal.form.farewellMessage") || "Farewell message"}</FormLabel>
                  <FormControl>
                    <Textarea rows={3} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <QueueSelect selectedQueueIds={selectedQueueIds} onChange={setSelectedQueueIds} />

            <DialogFooter>
              <Button type="button" variant="outline" onClick={onClose}>
                {i18n.t("whatsappModal.buttons.cancel")}
              </Button>
              <Button type="submit" loading={form.formState.isSubmitting}>
                {whatsAppId
                  ? i18n.t("whatsappModal.buttons.okEdit")
                  : i18n.t("whatsappModal.buttons.okAdd")}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
};

export default memo(WhatsAppModal);
