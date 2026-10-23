# Frontend

## Запуск
В корне директории frontend:

`npm run dev`

## Хук валидации репозитория

``` tsx
import { useRepositoryValidation } from '@/hooks/useRepositoryValidation';

const {
	url,        // string
	state,      // ValidationState (idle | pending | permanent-error | temporary-error | moved)
	isPending,  // boolean
	inputRef,   // RefObject<HTMLInputElement | null>
	setUrl,     // (value: string) => void
	submit,     // () => void
	retry,      // () => void
	fix,        // () => void
	useNew,     // () => void
} = useRepositoryValidation({
	onAnalysisCreated: (id) => navigate(`/analysis/${id}`),
});
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