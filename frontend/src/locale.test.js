import test from 'node:test';
import assert from 'node:assert/strict';
import { resolveLocale } from './locale.js';

test('uses the saved language before browser preferences', () => {
  assert.equal(resolveLocale('en', ['fr-FR']), 'en');
  assert.equal(resolveLocale('fr', ['en-US']), 'fr');
});

test('uses the first supported browser language and falls back to English', () => {
  assert.equal(resolveLocale(null, ['de-DE', 'fr-CA', 'en-US']), 'fr');
  assert.equal(resolveLocale(null, ['EN-us']), 'en');
  assert.equal(resolveLocale(null, ['de-DE']), 'en');
  assert.equal(resolveLocale('invalid', []), 'en');
});
