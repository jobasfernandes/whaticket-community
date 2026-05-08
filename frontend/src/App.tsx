import { ThemeProvider } from "next-themes";
import { Toaster } from "sonner";

import Routes from "./routes";

const App = () => {
  return (
    <ThemeProvider attribute="class" defaultTheme="system" enableSystem>
      <Routes />
      <Toaster position="top-right" richColors closeButton />
    </ThemeProvider>
  );
};

export default App;
