const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { test } = require('node:test');
const vm = require('node:vm');
const ts = require('typescript');
const source = readFileSync(path.join(__dirname, 'siviDateTimestamp.ts'), 'utf8');
const moduleExports = {};
vm.runInNewContext(ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 }
}).outputText, { exports: moduleExports });
const { siviDateTimestampError } = moduleExports;

test('reviewed Date preserves exact wallclock timestamps and nanosecond fractions without timezone inference', () => {
  for (const raw of ['0100-01-01 00:00:00', '9999-12-31 23:59:59.999999999',
    '2000-02-29 12:34:56', '2024-02-29 00:00:00.000',
    '2026-11-01 01:30:00.123456789', '2026-03-08 02:30:00.100000000']) {
    assert.equal(siviDateTimestampError(raw), null, raw);
  }
});

test('reviewed Date rejects incomplete, normalized, impossible and timezone-bearing inputs', () => {
  for (const raw of ['', '2026-01-01', '2026-01-01T00:00:00', ' 2026-01-01 00:00:00',
    '2026-01-01 00:00:00 ', '2026-01-01 00:00:00\n', '2026-01-01 00:00:00\r\n',
    '2026-01-01 00:00:00Z', '2026-01-01 00:00:00+01:00',
    '0099-12-31 23:59:59', '0000-01-01 00:00:00', '10000-01-01 00:00:00',
    '1900-02-29 00:00:00', '2025-02-29 00:00:00', '2026-04-31 00:00:00',
    '2026-00-01 00:00:00', '2026-13-01 00:00:00', '2026-01-00 00:00:00',
    '2026-01-32 00:00:00', '2026-01-01 24:00:00', '2026-01-01 00:60:00',
    '2026-01-01 00:00:60', '2026-01-01 00:00:00.', '2026-01-01 00:00:00.1234567890',
    '2026-01-01 00:00:00,1', '2026-01-01 00:00:00\0', '2026-01-01 00:00:00\ud800']) {
    assert.notEqual(siviDateTimestampError(raw), null, raw);
  }
});

test('Gregorian leap years and every calendar month are validated independently of local DST', () => {
  for (const year of [100, 400, 1900, 2000, 2024, 2100, 2400, 9999]) {
    const leap = year % 4 === 0 && (year % 100 !== 0 || year % 400 === 0);
    const days = [31, leap ? 29 : 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31];
    for (let month = 1; month <= 12; month++) {
      const prefix = `${String(year).padStart(4, '0')}-${String(month).padStart(2, '0')}-`;
      assert.equal(siviDateTimestampError(`${prefix}${days[month - 1]} 23:59:59.000000001`), null);
      assert.notEqual(siviDateTimestampError(`${prefix}${days[month - 1] + 1} 00:00:00`), null);
    }
  }
});
