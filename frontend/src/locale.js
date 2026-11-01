export const SUPPORTED_LOCALES = ['en', 'fr'];
export const LOCALE_STORAGE_KEY = 'capimgr.locale';

export function resolveLocale(preference, browserLanguages = []) {
  if (SUPPORTED_LOCALES.includes(preference)) return preference;
  for (const language of browserLanguages) {
    const base = language.toLowerCase().split('-')[0];
    if (SUPPORTED_LOCALES.includes(base)) return base;
  }
  return 'en';
}
