import { Loader2 } from "lucide-react";

const BackdropLoading = () => {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-background/80 backdrop-blur-sm">
      <Loader2 className="size-10 animate-spin text-primary" />
    </div>
  );
};

export default BackdropLoading;
