import { createRef } from 'react';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { Button, Input, Message } from './index';

afterEach(cleanup);

it('keeps an external hint and the field error accessible together', () => {
  const ref = createRef<HTMLInputElement>();
  render(
    <>
      <span id="external-hint">Подсказка</span>
      <Input ref={ref} label="URL" error="Неверный формат" aria-describedby="external-hint" />
    </>,
  );
  const input = screen.getByRole('textbox', { name: 'URL' });
  const ids = input.getAttribute('aria-describedby')!.split(' ');
  expect(ids).toHaveLength(2);
  expect(ids.map((id) => document.getElementById(id)?.textContent)).toEqual([
    'Подсказка',
    'Неверный формат',
  ]);
  expect(input.getAttribute('aria-invalid')).toBe('true');
  ref.current?.focus();
  expect(document.activeElement).toBe(input);
});

it('does not invoke a pending button twice or submit by default', () => {
  const click = vi.fn();
  render(
    <Button isLoading onClick={click}>
      Повторить
    </Button>,
  );
  const button = screen.getByRole('button', { name: 'Повторить' }) as HTMLButtonElement;
  expect(button.type).toBe('button');
  expect(button.disabled).toBe(true);
  fireEvent.click(button);
  fireEvent.click(button);
  expect(click).not.toHaveBeenCalled();
});

it.each([
  ['error', 'alert'],
  ['warning', 'alert'],
  ['info', 'status'],
  ['success', 'status'],
] as const)('announces a %s message as %s with its actions', (variant, role) => {
  const retry = vi.fn();
  render(
    <Message variant={variant} actions={<Button onClick={retry}>Повторить</Button>}>
      Результат проверки
    </Message>,
  );
  expect(screen.getByRole(role).textContent).toContain('Результат проверки');
  fireEvent.click(screen.getByRole('button', { name: 'Повторить' }));
  expect(retry).toHaveBeenCalledTimes(1);
});
