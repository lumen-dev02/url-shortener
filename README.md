# URL Shortener

Небольшой сервис сокращения ссылок на Go. Принимает исходный URL, создаёт короткий код, сохраняет соответствие в PostgreSQL и перенаправляет посетителя на исходный адрес.

## Возможности

- создание короткой ссылки через HTTP API;
- перенаправление по короткому коду;
- проверка формата URL перед сохранением;
- хранение данных в PostgreSQL;
- конфигурация через переменные окружения;
- graceful shutdown HTTP-сервера;
- unit-тесты бизнес-логики и HTTP-обработчиков.

## Стек

- Go 1.25+
- стандартная библиотека net/http
- PostgreSQL 16
- pgx v5
- Docker Compose

## Запуск

Понадобятся Go 1.25+, Docker и Docker Compose.

1. Скопируйте пример конфигурации: `cp .env.example .env`
2. Запустите PostgreSQL: `docker compose up -d postgres`
3. Примените миграцию:
   ```bash
   docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U shortener -d shortener < migrations/001_create_urls.up.sql
   ```
4. Загрузите зависимости, экспортируйте переменные из .env и запустите приложение:
   ```bash
   go mod download
   set -a && . ./.env && set +a
   go run ./cmd/api
   ```

Сервис будет доступен на http://localhost:8080.

## API

### Создать короткую ссылку

POST /api/v1/shorten

Тело запроса:
```json
{
  "original_url": "https://example.com/articles/go"
}
```

Успешный ответ — 201 Created:
```json
{
  "code": "aB3xYz",
  "short_url": "http://localhost:8080/aB3xYz",
  "original_url": "https://example.com/articles/go"
}
```

Ошибки:
- 400 Bad Request — некорректный JSON или URL;
- 405 Method Not Allowed — неподдерживаемый HTTP-метод;
- 500 Internal Server Error — внутренняя ошибка сервиса.

### Перейти по короткой ссылке

GET /{code}

При существующем коде сервер отвечает 302 Found и устанавливает заголовок Location с исходным URL. Если код не найден, возвращается 404 Not Found.

### Проверка состояния

GET /healthz

Возвращает 200 OK, если HTTP-сервер отвечает. Проверка не заменяет проверку доступности базы данных.

## Конфигурация

| Переменная | Назначение | Значение по умолчанию |
|---|---|---|
| HTTP_ADDR | Адрес HTTP-сервера | :8080 |
| DATABASE_URL | Строка подключения PostgreSQL | обязательна |
| BASE_URL | Публичная базовая часть короткой ссылки | http://localhost:8080 |

`BASE_URL` должен содержать HTTP(S)-адрес без логина, пароля, query-параметров и fragment.
Файл `.env` не загружается автоматически — перед запуском нужно экспортировать переменные, как показано выше.
Пароль БД из примера предназначен только для локальной разработки.

## Проверки

```bash
go test ./...
go test -race ./...
go vet ./...
```

Для интеграционного теста репозитория запустите PostgreSQL и задайте отдельную переменную:

```bash
TEST_DATABASE_URL='postgres://shortener:shortener@localhost:5432/shortener?sslmode=disable' go test -race ./...
```

Тест создаёт и удаляет собственную схему в указанной БД; пользователь должен иметь право `CREATE`.
Без `TEST_DATABASE_URL` этот тест пропускается. В GitHub Actions он запускается с PostgreSQL 16.

## Структура проекта

```text
cmd/api/             точка входа приложения
internal/config/     загрузка конфигурации
internal/handler/    HTTP-обработчики и маршруты
internal/service/    бизнес-логика сокращения ссылок
internal/repository/ работа с PostgreSQL
migrations/          SQL-миграции
```

## Ограничения текущей версии

- короткий код генерируется случайным образом;
- срок действия ссылок и пользовательские алиасы не поддерживаются;
- авторизация и статистика переходов не входят в текущий объём проекта;
- одинаковые исходные URL могут получать разные короткие ссылки;
- rate limiting не реализован: без дополнительной защиты сервис не стоит открывать для публичной записи.
