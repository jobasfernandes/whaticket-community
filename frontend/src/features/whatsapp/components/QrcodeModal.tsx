import { memo, useEffect, useState } from "react";
import { QRCodeSVG } from "qrcode.react";

import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Skeleton } from "@/components/ui/skeleton";
import openSocket from "@/lib/socket";
import toastError from "@/utils/toastError";
import { i18n } from "@/lib/i18n";
import { getWhatsapp } from "@/features/whatsapp/api/whatsapp";
import type { Whatsapp } from "@/types/domain";

interface Props {
  open: boolean;
  onClose: () => void;
  whatsAppId: number | null | undefined;
}

const QrcodeModal = ({ open, onClose, whatsAppId }: Props) => {
  const [qrCode, setQrCode] = useState("");

  useEffect(() => {
    if (!whatsAppId) return;
    (async () => {
      try {
        const data = await getWhatsapp(whatsAppId);
        setQrCode(data.qrcode ?? "");
      } catch (err) {
        toastError(err);
      }
    })();
  }, [whatsAppId]);

  useEffect(() => {
    if (!whatsAppId) return;
    const socket = openSocket();
    socket.on("whatsappSession", (raw: unknown) => {
      const evt = raw as { action?: string; session?: Whatsapp };
      if (evt?.action !== "update" || !evt.session || evt.session.id !== whatsAppId) return;
      setQrCode(evt.session.qrcode ?? "");
      if (evt.session.qrcode === "") onClose();
    });
    return () => {
      socket.disconnect();
    };
  }, [whatsAppId, onClose]);

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>{i18n.t("qrCode.message")}</DialogTitle>
        </DialogHeader>
        <div className="flex items-center justify-center py-6">
          {qrCode ? (
            <QRCodeSVG value={qrCode} size={256} />
          ) : (
            <Skeleton className="size-64" />
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
};

export default memo(QrcodeModal);
