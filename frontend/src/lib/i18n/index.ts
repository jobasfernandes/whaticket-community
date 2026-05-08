import i18n from "i18next";
import LanguageDetector from "i18next-browser-languagedetector";

import { messages } from "./languages";

void i18n.use(LanguageDetector).init({
  debug: false,
  defaultNS: "translations",
  fallbackLng: "en",
  ns: ["translations"],
  resources: messages,
  interpolation: {
    escapeValue: false,
  },
});

export { i18n };
