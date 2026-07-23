export type ThemeMode = 'system' | 'light' | 'dark';
export type ViewMode = 'focus' | 'board';

export const PREFERENCES_KEY = 'cc-present:preferences:v1';

export const PREFERENCES_SCHEMA = {
  identity: 'cc-present-web-preferences-v1',
  version: 1,
  fingerprint: '994e1aeeaf1ad191057a153f3a10f441c6de3a93a6c24c7ed7d4f558a4c14ef4',
} as const;

interface Preferences {
  theme: ThemeMode;
  views: Record<string, ViewMode>;
}

interface PreferenceStorage {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
}

const objectValue = (value: unknown, label: string): Record<string, unknown> => {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) {
    throw new Error(`${label} must be an object`);
  }
  return value as Record<string, unknown>;
};

const exactKeys = (value: unknown, keys: string[], label: string): Record<string, unknown> => {
  const object = objectValue(value, label);
  const actual = Object.keys(object).sort();
  const expected = [...keys].sort();
  if (actual.length !== expected.length || actual.some((key, index) => key !== expected[index])) {
    throw new Error(`${label} keys do not match exact schema v1`);
  }
  return object;
};

const defaultPreferences = (): Preferences => ({ theme: 'system', views: Object.create(null) as Record<string, ViewMode> });

const decodePreferences = (raw: string): Preferences => {
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw) as unknown;
  } catch (error) {
    throw new Error('cc-present preferences are corrupt', { cause: error });
  }
  const envelope = exactKeys(parsed, ['schema', 'theme', 'views'], 'preference envelope');
  const schema = exactKeys(envelope.schema, ['identity', 'version', 'fingerprint'], 'preference schema');
  if (
    schema.identity !== PREFERENCES_SCHEMA.identity ||
    schema.version !== PREFERENCES_SCHEMA.version ||
    schema.fingerprint !== PREFERENCES_SCHEMA.fingerprint
  ) {
    throw new Error('cc-present preference schema does not match exact v1');
  }
  const theme = envelope.theme;
  if (theme !== 'system' && theme !== 'light' && theme !== 'dark') {
    throw new Error('cc-present preference theme is invalid');
  }
  const rawViews = objectValue(envelope.views, 'preference views');
  const views = Object.create(null) as Record<string, ViewMode>;
  for (const [subject, mode] of Object.entries(rawViews)) {
    if (subject.length === 0 || (mode !== 'focus' && mode !== 'board')) {
      throw new Error('cc-present preference view is invalid');
    }
    views[subject] = mode;
  }
  const preferences: Preferences = { theme, views };
  if (encodePreferences(preferences) !== raw) {
    throw new Error('cc-present preferences are not exact canonical JSON');
  }
  return preferences;
};

const encodePreferences = (preferences: Preferences): string => {
  const views = Object.fromEntries(Object.entries(preferences.views).sort(([left], [right]) => left.localeCompare(right)));
  return JSON.stringify({ schema: PREFERENCES_SCHEMA, theme: preferences.theme, views });
};

const loadPreferences = (storage: PreferenceStorage = localStorage): Preferences => {
  const raw = storage.getItem(PREFERENCES_KEY);
  return raw === null ? defaultPreferences() : decodePreferences(raw);
};

const savePreferences = (preferences: Preferences, storage: PreferenceStorage = localStorage): void => {
  storage.setItem(PREFERENCES_KEY, encodePreferences(preferences));
};

export function loadTheme(storage: PreferenceStorage = localStorage): ThemeMode {
  return loadPreferences(storage).theme;
}

export function saveTheme(theme: ThemeMode, storage: PreferenceStorage = localStorage): void {
  const preferences = loadPreferences(storage);
  savePreferences({ ...preferences, theme }, storage);
}

export function loadView(subject: string, storage: PreferenceStorage = localStorage): ViewMode | null {
  const views = loadPreferences(storage).views;
  return Object.hasOwn(views, subject) ? (views[subject] ?? null) : null;
}

export function saveView(subject: string, mode: ViewMode, storage: PreferenceStorage = localStorage): void {
  if (subject.length === 0) throw new Error('cc-present preference subject is empty');
  const preferences = loadPreferences(storage);
  const views = Object.assign(Object.create(null) as Record<string, ViewMode>, preferences.views, { [subject]: mode });
  savePreferences({ ...preferences, views }, storage);
}
