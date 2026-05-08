import { toast } from "sonner";
import { isAxiosError } from "axios";
import { i18n } from "@/lib/i18n";

const toastError = (err: unknown): void => {
  let errorMsg: string | undefined;
  if (isAxiosError(err)) {
    const data = err.response?.data as { message?: string; error?: string } | undefined;
    errorMsg = data?.message ?? data?.error;
  }
  if (errorMsg) {
    if (i18n.exists(`backendErrors.${errorMsg}`)) {
      toast.error(i18n.t(`backendErrors.${errorMsg}`), { id: errorMsg });
    } else {
      toast.error(errorMsg, { id: errorMsg });
    }
    return;
  }
  toast.error("An error occurred!");
};

export default toastError;
