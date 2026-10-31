# CLAUDE.md

«Завертыш» — онлайн-игра для компании от 2 до 8 человек: игроки пишут вопросы,
загибают лист и отвечают вслепую на видимое начало чужого вопроса. Правила и
запуск — в README.md.

## Структура

- `api/openapi.yaml` — единственный источник контракта HTTP API и протокола
  WebSocket. Сначала меняется он, потом код.
- `backend/` — Go (gin, pgx, coder/websocket). Модуль `zvrtsh`, команды запускать из `backend/`.
  - `cmd/server` — точка входа, конфиг `config/config.yml` (любой ключ
    переопределяется env: `users-db.host` → `USERS_DB_HOST`).
  - `internal/model` — сущности и доменные ошибки (`ErrValidation`, `ErrNoSession`, …).
  - `internal/app` — правила игры (лобби, старт, ходы, голоса), хаб событий
    WebSocket (в памяти), очистка по TTL.
  - `internal/ports/httpserver` — реализация сгенерированного `StrictServerInterface`,
    маппинг ошибок в HTTP (`errors.go`), авторизация (`auth.go`), WebSocket (`events.go`).
  - `internal/ports/httpserver/api/api.gen.go` — сгенерирован, руками не править.
  - `internal/repo/*_repo` — интерфейсы репозиториев и фейки для тестов (`fake/`).
  - `internal/repo/postgres` — все реализации на одном `pgxpool.Pool`.
  - `migrations/` — миграции golang-migrate (`NNNNNN_name.up.sql` / `.down.sql`).
- `frontend/` — React 19 + TypeScript + Vite, без UI-библиотек и роутера (hash-роутинг
  в `src/lib/router.ts`), стили в одном `src/styles.css` с токенами в `:root`.
  `src/api/schema.d.ts` сгенерирован из контракта.

## Команды

Все через `make` из корня (нужны docker и `.env` из `.env.dist`):

- `make generate` — код бэкенда и фронтенда из `api/openapi.yaml`. Запускать после
  любой правки контракта и коммитить сгенерированное.
- `make test` — все проверки: `test-back` (пересоздаёт базу `zvrtsh_test`, гоняет
  миграции up → down → up, `go test -race` с тестами на Postgres) и `test-front`
  (`tsc` + сборка). `make test-unit` — Go-тесты без базы.
- `make run-all` — всё в docker: игра на :3000, API на :8080.
- `make run-back` / `make dev-back` / `make run-front` — по частям, фронтенд-dev на :5173.
- `make migrate-create name=...`, `make migrate-up`, `make migrate-down`.

## Соглашения

- Ошибки домена — переменные из `internal/model/errors.go`, оборачивать через
  `fmt.Errorf("%w: ...")`. HTTP-статус выбирается только в `httpserver/errors.go`;
  хендлеры возвращают ошибку как есть. Детали 500 только логируются.
- Новая операция API: контракт → `make generate` → метод в `handlers.go` → логика в `app`.
  Публичные (без токена) маршруты перечислены в `publicRoutes` (`auth.go`), тест
  сверяет их с операциями без `security` в контракте.
- Изменение партии — только через `SessionRepo.UpdateSession`: репозиторий блокирует
  строку (`FOR UPDATE`), применяет функцию с правилами из `app` и увеличивает `version`.
  После успешного изменения `app` публикует партию в хаб. Клиенты упорядочивают
  снимки по `version`. Отменённое лобби удаляется через `DeleteSession` и
  публикуется как `SessionEvent{Deleted: true}` (`session_deleted` в сокете).
- Партия начинается в лобби (`Started == false`, код приглашения хранится в ней);
  при старте `model.Deal` раздаёт листы на все шаги заранее (`Asker`/`Answerer` в
  каждом шаге). Листы перемешиваются только между вопросом и ответом: ответы
  открываются, когда все написали вопросы шага (`AnswersOpen`); следующий вопрос
  пишет на том же листе тот, кто на нём ответил (`Asker` шага = `Answerer` прошлого).
  Источник случайности — поле `newRand`, тесты подменяют его.
- Время в приложении — UTC с точностью до микросекунд (как в Postgres), берётся
  из поля `now`, чтобы тесты могли подменять часы.
- Схема БД меняется только новой миграцией с рабочим `.down.sql`; старые не править.
- Тексты интерфейса на русском; код, комментарии и сообщения API — на английском.
- Фронтенд не опрашивает сервер: состояние грузится через REST при открытии и
  после каждого переподключения сокета (`syncKey`), дальше обновляется событиями;
  опрос включается только при `status === "offline"`.

## Подводные камни

- WebSocket проверяет, что `Origin` совпадает с `Host`: в nginx нужен
  `proxy_set_header Host $http_host` (с портом), в Vite — прокси без `changeOrigin`.
- Чтобы играть за нескольких в одном браузере, используйте вкладки `localhost:3000`,
  `127.0.0.1:3000` и адрес в локальной сети (`ipconfig getifaddr en0`) — у них разное
  `localStorage`.
- `docker compose up --build <сервис>` пересобирает и зависимости; чтобы обновить
  только фронтенд — `--no-deps`.
- Бэкенд рассчитан на один экземпляр: хаб событий живёт в памяти процесса.
