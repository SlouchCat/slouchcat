import { describe, expect, it } from 'vitest';
import { validPort } from './shared';

describe('validPort', () => {
  it('takes digits from 1 to 65535, blanks around them aside', () => {
    for (const text of ['1', '25', '587', ' 465 ', '65535', '00025']) {
      expect(validPort(text), text).toBe(true);
    }
  });

  it('rejects what Number() would read as a port but nobody types as one', () => {
    for (const text of [
      '0x19',
      '1e3',
      '25.0',
      '+25',
      '-1',
      '0',
      '65536',
      '123456',
      '',
      ' ',
      'abc',
    ]) {
      expect(validPort(text), text).toBe(false);
    }
  });
});
