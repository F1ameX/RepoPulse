export function validateCredentials(rawLogin: string, rawPassword: string) {
  const login = rawLogin.replace(/^ +| +(?![\s\S])/g, '');
  const password = rawPassword.normalize('NFC');
  const errors: { login?: string; password?: string } = {};

  if (!login) errors.login = 'Укажите логин';
  else if ([...login].length < 3 || [...login].length > 32)
    errors.login = 'Логин должен содержать от 3 до 32 символов';
  else if (!/^[A-Za-z][A-Za-z0-9_]*$/.test(login) || /[^A-Za-z0-9_]/.test(login))
    errors.login =
      'Начните логин с английской буквы. Дальше можно использовать английские буквы, цифры и знак подчёркивания (_).';

  const length = [...password].length;
  if (!password) errors.password = 'Укажите пароль';
  else if (length < 15 || length > 128)
    errors.password = 'Пароль должен содержать от 15 до 128 символов';
  else if (/[^\p{L}\p{M}\p{N}\p{P}\p{S} ]/u.test(password))
    errors.password =
      'Используйте буквы, цифры, знаки и обычные пробелы. Уберите табуляцию, переносы строк и другие невидимые символы.';
  else if (/^ +$/.test(password)) errors.password = 'Пароль не может состоять только из пробелов';

  return { credentials: { login: login.toLowerCase(), password }, errors };
}
