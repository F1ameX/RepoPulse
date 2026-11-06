import { expect, it } from 'vitest';
import { validateCredentials } from './credentials';

it('normalizes ASCII login and NFC password without trimming password spaces', () => {
  const result = validateCredentials(' FlameX ', '  abcdefghijkle\u0301  ');
  expect(result.errors).toEqual({});
  expect(result.credentials).toEqual({ login: 'flamex', password: '  abcdefghijklé  ' });
});

it.each([2, 3, 32, 33])('checks login length %s', (length) => {
  const result = validateCredentials('a'.repeat(length), 'a'.repeat(15));
  expect(Boolean(result.errors.login)).toBe(length < 3 || length > 32);
});

it.each([14, 15, 128, 129])('checks password length %s in code points', (length) => {
  const result = validateCredentials('abc', '🔐'.repeat(length));
  expect(Boolean(result.errors.password)).toBe(length < 15 || length > 128);
});

it('rejects tabs and line breaks in both credentials', () => {
  const { errors } = validateCredentials('alice\n', 'a'.repeat(15) + '\t');
  expect(errors.login).toContain('Начните логин с английской буквы');
  expect(errors.password).toContain('Уберите табуляцию');
});
