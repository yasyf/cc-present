// Pure helpers for ChartView: a baseline-anchored nice-tick scale, compact value
// formatting, and a deterministic categorical series palette rotated off --accent.

export type ChartMode = 'light' | 'dark';

export interface Scale {
  min: number;
  max: number;
  ticks: number[];
}

export interface BarLayout {
  groupW: number;
  gap: number;
  barW: number;
}

// niceScale rounds the data range out to clean ticks, always including the baseline
// 0: bars and lines both grow from 0, and a negative value extends the domain below it.
export function niceScale(dataMin: number, dataMax: number, targetCount = 5): Scale {
  const lo = Math.min(0, dataMin);
  let hi = Math.max(0, dataMax);
  if (lo === hi) hi = lo + 1;
  const step = niceNum(niceNum(hi - lo, false) / (targetCount - 1), true);
  // A range too small to resolve a positive nice step (denormal underflow) or an
  // overflowed one collapses to a unit scale rather than emit NaN coordinates.
  if (!Number.isFinite(step) || step <= 0) return { min: lo, max: lo + 1, ticks: [lo, lo + 1] };
  const min = Math.floor(lo / step) * step;
  const max = Math.ceil(hi / step) * step;
  const decimals = Math.max(0, -Math.floor(Math.log10(step)));
  const ticks: number[] = [];
  for (let v = min; v <= max + step / 2; v += step) ticks.push(roundTo(v, decimals));
  return { min, max, ticks };
}

function niceNum(range: number, round: boolean): number {
  const exp = Math.floor(Math.log10(range));
  const frac = range / 10 ** exp;
  const nice = round
    ? frac < 1.5
      ? 1
      : frac < 3
        ? 2
        : frac < 7
          ? 5
          : 10
    : frac <= 1
      ? 1
      : frac <= 2
        ? 2
        : frac <= 5
          ? 5
          : 10;
  return nice * 10 ** exp;
}

function roundTo(value: number, decimals: number): number {
  const f = 10 ** decimals;
  return Math.round(value * f) / f;
}

const compact = new Intl.NumberFormat('en-US', { notation: 'compact', maximumFractionDigits: 1 });
const precise = new Intl.NumberFormat('en-US', { maximumSignificantDigits: 2 });

// formatValue labels a tick or mark: a sub-unit magnitude keeps two significant digits so
// 0.01 and 0.02 stay distinct, a larger one compacts (1.3K, 4.2M). Appends the unit.
export function formatValue(value: number, unit?: string): string {
  const n = value !== 0 && Math.abs(value) < 1 ? precise.format(value) : compact.format(value);
  return unit ? `${n} ${unit}` : n;
}

// barLayout sizes a category's grouped bars: the group spans at most 80% of the band (and
// no more than n full bars), gaps shrink with the band, and the bar width floors above 0
// so the documented 100×6 cap can't drive negative, overlapping bars.
export function barLayout(bandW: number, n: number, maxBar = 24, maxGap = 2): BarLayout {
  const groupW = Math.min(bandW * 0.8, n * maxBar + (n - 1) * maxGap);
  const gap = Math.min(maxGap, bandW * 0.05);
  const barW = Math.max(0.5, (groupW - (n - 1) * gap) / n);
  return { groupW, gap, barW };
}

// Slot 1 is --accent verbatim; slots 2..6 walk HUE_OFFSETS off the accent hue at the
// mode's fixed OKLCH lightness/chroma. Validated with the dataviz palette checks.
const DERIVED = {
  light: { l: 0.62, c: 0.13 },
  dark: { l: 0.6, c: 0.13 },
} as const;
const HUE_OFFSETS = [108, 324, 180, 36, 252];

// seriesColors returns count fill colors: the resolved accent verbatim as slot 1, then
// deterministic hue-rotated derivations at the mode's fixed lightness/chroma.
export function seriesColors(accent: string, mode: ChartMode, count: number): string[] {
  const hue = accentHue(accent);
  const { l, c } = DERIVED[mode];
  const colors = [accent];
  for (const offset of HUE_OFFSETS.slice(0, count - 1)) colors.push(oklchToHex(l, c, hue + offset));
  return colors;
}

function accentHue(color: string): number {
  const [, a, b] = linearToOklab(parseColor(color).map(srgbToLinear) as [number, number, number]);
  return ((Math.atan2(b, a) * 180) / Math.PI + 360) % 360;
}

function parseColor(color: string): [number, number, number] {
  if (/^rgba?\(/.test(color)) {
    const n = color.match(/-?\d*\.?\d+/g) ?? [];
    return [Number(n[0]) / 255, Number(n[1]) / 255, Number(n[2]) / 255];
  }
  const h = color.replace(/^#/, '');
  const hex = h.length === 3 ? [...h].map((ch) => ch + ch).join('') : h;
  return [0, 2, 4].map((i) => parseInt(hex.slice(i, i + 2), 16) / 255) as [number, number, number];
}

function srgbToLinear(c: number): number {
  return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
}

function linearToSrgb(c: number): number {
  const x = Math.max(0, Math.min(1, c));
  return x <= 0.0031308 ? 12.92 * x : 1.055 * x ** (1 / 2.4) - 0.055;
}

function linearToOklab([r, g, b]: [number, number, number]): [number, number, number] {
  const l = Math.cbrt(0.4122214708 * r + 0.5363325363 * g + 0.0514459929 * b);
  const m = Math.cbrt(0.2119034982 * r + 0.6806995451 * g + 0.1073969566 * b);
  const s = Math.cbrt(0.0883024619 * r + 0.2817188376 * g + 0.6299787005 * b);
  return [
    0.2104542553 * l + 0.793617785 * m - 0.0040720468 * s,
    1.9779984951 * l - 2.428592205 * m + 0.4505937099 * s,
    0.0259040371 * l + 0.7827717662 * m - 0.808675766 * s,
  ];
}

function oklabToLinear([l, a, b]: [number, number, number]): [number, number, number] {
  const l_ = (l + 0.3963377774 * a + 0.2158037573 * b) ** 3;
  const m_ = (l - 0.1055613458 * a - 0.0638541728 * b) ** 3;
  const s_ = (l - 0.0894841775 * a - 1.291485548 * b) ** 3;
  return [
    4.0767416621 * l_ - 3.3077115913 * m_ + 0.2309699292 * s_,
    -1.2684380046 * l_ + 2.6097574011 * m_ - 0.3413193965 * s_,
    -0.0041960863 * l_ - 0.7034186147 * m_ + 1.707614701 * s_,
  ];
}

function inGamut([r, g, b]: [number, number, number]): boolean {
  return r >= -0.001 && r <= 1.001 && g >= -0.001 && g <= 1.001 && b >= -0.001 && b <= 1.001;
}

// oklchToHex clamps chroma inward until the color falls inside sRGB, holding lightness
// and hue, so a rotated slot never renders as an out-of-gamut clip.
function oklchToHex(l: number, c: number, hueDeg: number): string {
  const h = (hueDeg * Math.PI) / 180;
  let lo = 0;
  let hi = c;
  for (let i = 0; i < 24; i++) {
    const mid = (lo + hi) / 2;
    if (inGamut(oklabToLinear([l, mid * Math.cos(h), mid * Math.sin(h)]))) lo = mid;
    else hi = mid;
  }
  const [lr, lg, lb] = oklabToLinear([l, lo * Math.cos(h), lo * Math.sin(h)]);
  const byte = (v: number) =>
    Math.round(linearToSrgb(v) * 255)
      .toString(16)
      .padStart(2, '0');
  return `#${byte(lr)}${byte(lg)}${byte(lb)}`;
}
