import { describe, expect, it } from 'vitest';
import { sanitizeForSgr, stripAnsi } from './ansi';

const ESC = String.fromCharCode(27);
const BEL = String.fromCharCode(7);

describe('stripAnsi', () => {
  it('removes semicolon-form SGR', () => {
    expect(stripAnsi(`${ESC}[1;32mbold green${ESC}[0m`)).toBe('bold green');
  });

  it('removes colon-form (truecolor) SGR', () => {
    expect(stripAnsi(`${ESC}[38:2::255:0:0mred${ESC}[0m`)).toBe('red');
  });

  it('removes cursor and erase CSI sequences', () => {
    expect(stripAnsi(`a${ESC}[2K${ESC}[1Ab`)).toBe('ab');
  });

  it('strips an ST-terminated OSC-8 hyperlink, keeping its visible text', () => {
    expect(stripAnsi(`${ESC}]8;;https://example.com${ESC}\\Link text${ESC}]8;;${ESC}\\`)).toBe(
      'Link text',
    );
  });

  it('strips a BEL-terminated OSC-8 hyperlink, keeping its visible text', () => {
    expect(stripAnsi(`${ESC}]8;;https://x.com${BEL}Link${ESC}]8;;${BEL}`)).toBe('Link');
  });

  it('strips an OSC window-title sequence', () => {
    expect(stripAnsi(`${ESC}]0;my title${BEL}rest`)).toBe('rest');
  });

  it('strips a lone ESC charset-select sequence', () => {
    expect(stripAnsi(`${ESC}(Bplain`)).toBe('plain');
  });

  it('leaves plain text untouched', () => {
    expect(stripAnsi('a plain line')).toBe('a plain line');
  });
});

describe('sanitizeForSgr', () => {
  it('keeps semicolon-form SGR runs', () => {
    expect(sanitizeForSgr(`${ESC}[31mred${ESC}[0m`)).toBe(`${ESC}[31mred${ESC}[0m`);
  });

  it('keeps colon-form SGR runs', () => {
    expect(sanitizeForSgr(`${ESC}[38:2::255:0:0mred${ESC}[0m`)).toBe(
      `${ESC}[38:2::255:0:0mred${ESC}[0m`,
    );
  });

  it('drops cursor and erase CSI, keeping the surrounding text', () => {
    expect(sanitizeForSgr(`a${ESC}[2K${ESC}[1Ab`)).toBe('ab');
  });

  it('drops OSC strings, keeping OSC-8 link text', () => {
    expect(sanitizeForSgr(`${ESC}]8;;https://x.com${ESC}\\Link${ESC}]8;;${ESC}\\`)).toBe('Link');
  });

  it('normalizes the parameterless reset ESC[m to ESC[0m', () => {
    expect(sanitizeForSgr(`${ESC}[31mred${ESC}[mplain`)).toBe(`${ESC}[31mred${ESC}[0mplain`);
  });
});
