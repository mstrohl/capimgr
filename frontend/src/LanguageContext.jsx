import React, { createContext, useContext, useEffect, useMemo, useState } from 'react';
import { LOCALE_STORAGE_KEY, resolveLocale } from './locale.js';

const LanguageContext = createContext(null);

function initialLanguage() {
  let saved = null;
  try {
    saved = localStorage.getItem(LOCALE_STORAGE_KEY);
  } catch (error) {
    console.warn('Unable to read the saved language preference:', error);
  }
  return resolveLocale(saved, navigator.languages || [navigator.language]);
}

export function LanguageProvider({ children }) {
  const [language, setLanguage] = useState(initialLanguage);
  useEffect(() => {
    document.documentElement.lang = language;
  }, [language]);
  const value = useMemo(() => ({
    language,
    t: (en, fr) => language === 'fr' ? fr : en,
    changeLanguage: (next) => {
      if (next !== 'en' && next !== 'fr') return;
      setLanguage(next);
      try {
        localStorage.setItem(LOCALE_STORAGE_KEY, next);
      } catch (error) {
        console.warn('Unable to save the language preference:', error);
      }
    },
  }), [language]);
  return <LanguageContext.Provider value={value}>{children}</LanguageContext.Provider>;
}

export function useLanguage() {
  return useContext(LanguageContext);
}
