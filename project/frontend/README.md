# Frontend

## Запуск

В корне директории frontend:

`npm ci`, затем `npm run dev`. API должен быть доступен на порту 8080.

## Проверки

`npm run lint`, `npm test`, `npm run build`.

Тесты проверяют правила URL из FR-01 и состояния формы из FR-02/FR-27,
включая границы длины, порядок ошибок, повторную попытку, гостевую сессию,
отключённые cookie и отмену запроса. Ответы сервисов в компонентных тестах
контролируются тестом; это не проверка доступности реального GitHub.
HTTP-клиент дополнительно проверяется на локальном тестовом сервере:
JSON-ошибки, HTML-ответ 502, разрыв соединения, тайм-аут и отмена запроса.

| Требование С1 | Проверки |
| --- | --- |
| FR-01: URL, 2048 кодовых точек, порядок ошибок | `src/lib/repositoryUrl.test.ts` |
| FR-01: ошибка у поля, ввод и фокус, запрет отправки | `src/pages/RepositoryInputPage.test.tsx` |
| FR-02: проверка, перенос, недоступность, временный отказ и повтор | `src/pages/RepositoryInputPage.test.tsx`, `src/services/repositoryService.test.ts` |
| FR-27: создание и подтверждение гостевой сессии, cookie, запрет незаметной смены аккаунта | `src/pages/RepositoryInputPage.test.tsx` |
| Input/Button/Message: доступные ошибки и блокировка повторного действия | `src/components/Controls.test.tsx` |
| Ошибки HTTP и отсутствие фиктивного состояния анализа | `src/services/api.test.ts`, `src/pages/PlaceholderPages.test.tsx` |

## Граница С1

Форма проверяет URL локально, подтверждает гостевой или учётный контекст
и проверяет доступность публичного репозитория через Gateway. Создание
анализа, отслеживание состояния, отчёт и интерфейс входа реализуются в
последующих спринтах. Успешная проверка URL не означает выполненный анализ.

## Хук валидации репозитория

``` tsx
import { useRepositoryValidation } from '@/hooks/useRepositoryValidation';

const {
	url,        // string
	state,      // idle | pending | field-error | unavailable | temporary-error | session-error | validated
	isPending,  // boolean
	inputRef,   // RefObject<HTMLInputElement | null>
	setUrl,     // (value: string) => void
	submit,     // () => void
	retry,      // () => void
	fix,        // () => void
} = useRepositoryValidation();
```

## Компоненты

### Импорт
На любой странице:

`import { Button, Input, Message } from '@/components';`

### Применение

#### Button

``` tsx
<Button
	// свои пропсы
	variant="primary" | "secondary" | "ghost"
	size="sm" | "md" | "lg"
	isLoading={true | false}
	fullWidth={true | false}
	leftIcon={<SearchIcon />}
	rightIcon="→"
	ref={btnRef}

	// всё от базового <button>, что наследуется через ButtonHTMLAttributes
	type="button" | "submit" | "reset"
	onClick={handleClick}
	onMouseEnter / onMouseLeave / onFocus / onBlur
	disabled
	form="my-form-id"
	name="action"
	value="delete"
	title="Подсказка"
	autoFocus
	tabIndex={-1}
	id="submit-btn"
	className="my-class"
	style={{ marginTop: 8 }}
	aria-label="Отправить"
	aria-pressed={true}
	data-testid="submit-btn"

	// children
>
	Текст кнопки
</Button>
```

#### Input

``` tsx
<Input
	// свои пропсы
	label="URL репозитория"
	hint="Поддерживается публичный GitHub"
	error="Репозиторий не найден"
	size="sm" | "md" | "lg"
	ref={inputRef}

	// всё от базового <input>, кроме нативного size (Omit<..., 'size'>)
	type="text" | "url" | "email" | "password" | "number" | "search"
	value={url}
	onChange={(e) => setUrl(e.target.value)}
	placeholder="https://github.com/user/repo"
	required
	disabled
	readOnly
	minLength={3}
	maxLength={200}
	pattern="^https?://.*"
	min / max / step
	autoComplete="url" | "email" | "off"
	inputMode="url" | "email" | "numeric"
	name="repo-url"
	id="repo-input"
	className="my-class"
	style={{ width: 320 }}
	aria-label="URL репозитория"
	aria-describedby="external-hint"
	data-testid="repo-input"

	// children не принимает
/>
```

#### Message

``` tsx
<Message
	variant="info" | "success" | "warning" | "error"
	onDismiss={() => setError('')}   // если не передать — нет кнопки ×
	className="my-class"             // для отступов/ширины снаружи
	icon={<CustomIcon />}
	// или:
	// icon="☺"
	// icon={false}
	// icon={undefined}
	actions={
		<Button variant="secondary" size="sm" onClick={fix}>
			Изменить URL
		</Button>
	}
>
	Текст сообщения
</Message>
```
