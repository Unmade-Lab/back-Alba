# Alba CRM — Полный гайд для фронтенд-разработчика

> Этот документ — твоя единственная инструкция. Здесь описано ВСЁ: каждый запрос, каждый ответ, каждый сценарий.
> Читай сверху вниз. Если что-то непонятно — перечитай. Здесь нет ничего лишнего.

---

## Оглавление

1. [Как устроена система (общая картина)](#1-как-устроена-система)
2. [Базовые правила работы с API](#2-базовые-правила)
3. [Авторизация — пошагово](#3-авторизация)
4. [Что показывать после логина (Онбординг vs Дашборд)](#4-что-показывать-после-логина)
5. [Чат — полный цикл](#5-чат--полный-цикл)
6. [WebSocket — подключение и обработка событий](#6-websocket)
7. [Виджеты — что рендерить под сообщениями ИИ](#7-виджеты)
8. [Уведомления (Push-нотификации)](#8-уведомления)
9. [Обработка ошибок](#9-обработка-ошибок)
10. [Полная структура приложения (что куда класть)](#10-структура-приложения)
11. [Чеклист — что сделать по порядку](#11-чеклист)

---

## 1. Как устроена система

Альба — это AI CRM. Пользователь общается с ИИ-ассистентом (Альбой) в чате. Альба умеет:
- Создавать сделки, компании, задачи
- Показывать аналитику по воронке продаж
- Генерировать бизнес-отчёты
- Обогащать данные о компаниях

**У нас два канала связи с сервером:**

```
┌─────────────────────────────────────────────────────────────────────┐
│                                                                     │
│    ФРОНТЕНД                                                        │
│                                                                     │
│    ┌─────────────────────────────────────────────────────────────┐  │
│    │  HTTP REST API                                              │  │
│    │  Для ВСЕХ запросов: логин, отправка сообщений, получение   │  │
│    │  истории, создание сессий и т.д.                           │  │
│    │  Base URL: https://your-api.com/api/v1                     │  │
│    └─────────────────────────────────────────────────────────────┘  │
│                                                                     │
│    ┌─────────────────────────────────────────────────────────────┐  │
│    │  WebSocket                                                  │  │
│    │  ТОЛЬКО для получения данных от сервера в реальном времени: │  │
│    │  стриминг ответов ИИ и push-уведомления.                   │  │
│    │  URL: wss://your-api.com/ws/chat?session_id=...            │  │
│    └─────────────────────────────────────────────────────────────┘  │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

**Важно понять:**
- Сообщения **отправляешь** через HTTP (`POST /chat/message`)
- Ответы от ИИ **получаешь** через WebSocket (стриминг текста по кускам)
- Через WebSocket **ничего не отправляешь** (он только на приём)

---

## 2. Базовые правила

### Заголовки

Каждый запрос (кроме `/auth/register`, `/auth/login`, `/auth/activate`, `/auth/refresh`) требует заголовок:

```
Authorization: Bearer eyJhbGciOiJIUzI1NiIs...
```

### Формат данных

- Все запросы и ответы в формате JSON
- Всегда отправляй заголовок `Content-Type: application/json`

### workspace_id

**НЕ НУЖНО** передавать `workspace_id` в запросах. Сервер сам достаёт его из JWT-токена.

---

## 3. Авторизация

### 3.1. Регистрация нового аккаунта

Это первое действие нового пользователя. Он создаёт свой «воркспейс» (компанию) и становится администратором.

**Куда стучать:** `POST /api/v1/auth/register`

**Что отправить:**
```json
{
  "workspace_name": "ООО Ромашка",
  "user_name":      "Иван Петров",
  "email":          "ivan@romashka.ru",
  "password":       "moysupersecret"
}
```

**Валидация (проверь на фронте перед отправкой):**
- `workspace_name` — обязательно, непустая строка
- `user_name` — обязательно, непустая строка
- `email` — обязательно, валидный email
- `password` — обязательно, минимум 6 символов

**Что придёт в ответ (статус `201 Created`):**
```json
{
  "access_token":  "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoiYTFiMmMz...",
  "refresh_token": "a4f8e2d3b7c1...",
  "user": {
    "id":           "550e8400-e29b-41d4-a716-446655440000",
    "name":         "Иван Петров",
    "email":        "ivan@romashka.ru",
    "role":         "admin",
    "workspace_id": "661f9511-f30c-42a8-b960-123456789abc"
  },
  "workspace": {
    "id":   "661f9511-f30c-42a8-b960-123456789abc",
    "name": "ООО Ромашка"
  }
}
```

**Что сделать после получения ответа:**
1. Сохрани `access_token` в переменную (в памяти, НЕ в localStorage)
2. Сохрани `refresh_token` в localStorage
3. Сохрани объект `user` в состояние приложения
4. Перенаправь пользователя на экран чата → сервер **уже создал** первую сессию чата с автоматическим приветствием

**Возможные ошибки:**

| Статус | Тело ответа | Что произошло |
|---|---|---|
| `400` | `{"error": "Invalid request parameters"}` | Не заполнены обязательные поля или email невалидный |
| `409` | `{"error": "User with this email already exists"}` | Такой email уже зарегистрирован |
| `500` | `{"error": "..."}` | Ошибка сервера, покажи «Попробуйте позже» |

---

### 3.2. Вход в существующий аккаунт

**Куда стучать:** `POST /api/v1/auth/login`

**Что отправить:**
```json
{
  "email":    "ivan@romashka.ru",
  "password": "moysupersecret"
}
```

**Что придёт в ответ (статус `200 OK`):**
```json
{
  "access_token":  "eyJhbGciOiJIUzI1...",
  "refresh_token": "b5e9f3a4c8d2...",
  "user": {
    "id":           "550e8400-e29b-41d4-a716-446655440000",
    "name":         "Иван Петров",
    "email":        "ivan@romashka.ru",
    "role":         "admin",
    "workspace_id": "661f9511-f30c-42a8-b960-123456789abc"
  }
}
```

> ⚠️ **Обрати внимание**: в ответе Login НЕТ объекта `workspace` (в отличие от Register). Если нужно имя воркспейса — запроси `GET /workspace/status` после логина.

**Что сделать после получения ответа:**
1. Сохрани токены так же, как при регистрации
2. Запроси `GET /api/v1/workspace/status` (раздел 4) чтобы понять, какой экран показывать

**Возможные ошибки:**

| Статус | Тело ответа | Что произошло |
|---|---|---|
| `400` | `{"error": "Invalid request: valid email and password are required"}` | Не заполнены поля |
| `401` | `{"error": "Invalid email or password"}` | Неверный email или пароль |

---

### 3.3. Обновление токена (refresh)

Access token живёт **15 минут**. Когда он протухает, любой запрос вернёт `401`. В этот момент нужно обновить токен.

**Куда стучать:** `POST /api/v1/auth/refresh`

**Что отправить:**
```json
{
  "refresh_token": "a4f8e2d3b7c1..."
}
```

**Что придёт в ответ (статус `200 OK`):**
```json
{
  "access_token":  "eyJhbGciOiJIUzI1NiIs...",
  "refresh_token": "НОВЫЙ_REFRESH_TOKEN_c7d3e..."
}
```

> ⚠️ **КРИТИЧЕСКИ ВАЖНО**: Refresh Token **одноразовый**! После использования старый удаляется из базы. Ты ОБЯЗАН сохранить новый `refresh_token` из ответа. Если потеряешь — пользователю придётся заново логиниться.

**Полный код автообновления (копируй и используй):**

```typescript
class ApiClient {
  private accessToken: string | null = null;
  private baseUrl: string;

  constructor(baseUrl: string) {
    this.baseUrl = baseUrl;
  }

  // Вызывается после логина/регистрации
  setTokens(access: string, refresh: string) {
    this.accessToken = access;
    localStorage.setItem('alba_refresh_token', refresh);
  }

  // Основной метод для всех запросов
  async request(path: string, options: RequestInit = {}): Promise<any> {
    // Первая попытка
    let response = await fetch(`${this.baseUrl}${path}`, {
      ...options,
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${this.accessToken}`,
        ...options.headers,
      },
    });

    // Если 401 — пробуем обновить токен и повторить запрос
    if (response.status === 401) {
      const refreshed = await this.refreshTokens();
      if (!refreshed) {
        // Refresh тоже протух — кидаем пользователя на логин
        window.location.href = '/login';
        throw new Error('Session expired');
      }

      // Повторяем оригинальный запрос с новым токеном
      response = await fetch(`${this.baseUrl}${path}`, {
        ...options,
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${this.accessToken}`,
          ...options.headers,
        },
      });
    }

    if (!response.ok) {
      const err = await response.json().catch(() => ({ error: 'Unknown error' }));
      throw new Error(err.error || `HTTP ${response.status}`);
    }

    return response.json();
  }

  private async refreshTokens(): Promise<boolean> {
    const refreshToken = localStorage.getItem('alba_refresh_token');
    if (!refreshToken) return false;

    try {
      const response = await fetch(`${this.baseUrl}/api/v1/auth/refresh`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ refresh_token: refreshToken }),
      });

      if (!response.ok) return false;

      const data = await response.json();
      this.setTokens(data.access_token, data.refresh_token);
      return true;
    } catch {
      return false;
    }
  }

  clear() {
    this.accessToken = null;
    localStorage.removeItem('alba_refresh_token');
  }
}

// Использование:
const api = new ApiClient('https://api.your-domain.com');

// После логина:
api.setTokens(loginResponse.access_token, loginResponse.refresh_token);

// Любой запрос:
const sessions = await api.request('/api/v1/chat/sessions');
```

---

### 3.4. Выход (logout)

**Куда стучать:** `POST /api/v1/auth/logout`

**Что отправить:**
```json
{
  "refresh_token": "текущий_refresh_token"
}
```

**Что придёт в ответ (статус `200 OK`):**
```json
{
  "message": "Logged out successfully"
}
```

**Что сделать после:**
1. Удалить все токены из памяти и localStorage
2. Перенаправить на экран логина

---

### 3.5. Активация приглашения (для сотрудников)

Когда администратор через Альбу приглашает сотрудника, тот получает ссылку с токеном. Сотрудник открывает ссылку и задаёт себе пароль.

**Куда стучать:** `POST /api/v1/auth/activate`

**Что отправить:**
```json
{
  "token":    "abc123def456",
  "password": "moynewpassword"
}
```

**Что придёт в ответ (статус `200 OK`):**
```json
{
  "message":      "Account created and activated successfully",
  "email":        "colleague@romashka.ru",
  "role":         "employee",
  "workspace_id": "661f9511-...",
  "user_id":      "user-uuid-..."
}
```

> После активации пользователь должен залогиниться через `/auth/login`.

---

## 4. Что показывать после логина

Сразу после логина нужно понять: это **новый** пользователь (ещё не прошёл настройку) или **старый** (уже рабоает в CRM)?

**Куда стучать:** `GET /api/v1/workspace/status`

**Что придёт в ответ (статус `200 OK`):**
```json
{
  "id":                   "661f9511-f30c-42a8-b960-123456789abc",
  "name":                 "ООО Ромашка",
  "onboarding_completed": false
}
```

### Алгоритм (ОБЯЗАТЕЛЬНО реализуй):

```
onboarding_completed === false?
│
├── ДА (false) → Показать ПОЛНОЭКРАННЫЙ ЧАТ
│                 Альба задаст вопросы о бизнесе пользователя.
│                 Когда онбординг завершится, Альба вернёт intent = "complete_onboarding".
│                 В этот момент → перезапроси /workspace/status → onboarding_completed станет true.
│                 → Переключись на дашборд.
│
└── НЕТ (true) → Показать CRM ДАШБОРД
                   Боковая панель с чатами + основная область.
```

**Визуально:**

```
┌──────────────────────────────────────────────────────────────┐
│  ЭКРАН ОНБОРДИНГА (onboarding_completed = false)             │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐  │
│  │                                                        │  │
│  │    Альба: Привет! Я помогу настроить твою CRM.        │  │
│  │    Расскажи, чем занимается твоя компания?             │  │
│  │                                                        │  │
│  │    Ты: Мы продаём B2B SaaS для логистики              │  │
│  │                                                        │  │
│  │    Альба: Отлично! Какие этапы в вашей воронке?        │  │
│  │    ...                                                 │  │
│  │                                                        │  │
│  │    ┌─────────────────────────────────────────────────┐ │  │
│  │    │  Введите сообщение...                    [➤]    │ │  │
│  │    └─────────────────────────────────────────────────┘ │  │
│  └────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────┘

                         ↓ После онбординга ↓

┌──────────────────────────────────────────────────────────────┐
│  CRM ДАШБОРД (onboarding_completed = true)                   │
│                                                              │
│  ┌─────────────┐  ┌──────────────────────────────────────┐  │
│  │  САЙДБАР     │  │  ЧАТ                                 │  │
│  │              │  │                                       │  │
│  │ [+Новый чат] │  │  Альба: Чем могу помочь?              │  │
│  │              │  │                                       │  │
│  │ ● Настройка  │  │  Ты: Покажи все сделки               │  │
│  │   воронки    │  │                                       │  │
│  │              │  │  Альба: Вот 5 сделок:                 │  │
│  │ ● Сделка     │  │  ┌──────────────────────────────────┐│  │
│  │   Tesla      │  │  │  ВИДЖЕТ: Список сделок           ││  │
│  │              │  │  │  Tesla — $500k — Demo             ││  │
│  │ ● Аналитика  │  │  │  Apple — $200k — Lead             ││  │
│  │              │  │  └──────────────────────────────────┘│  │
│  │              │  │                                       │  │
│  │              │  │  ┌─────────────────────────────────┐ │  │
│  │              │  │  │  Введите сообщение...      [➤]  │ │  │
│  │              │  │  └─────────────────────────────────┘ │  │
│  └─────────────┘  └──────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────┘
```

---

## 5. Чат — полный цикл

### 5.1. Получить список всех чатов (для сайдбара)

Этот запрос делай при загрузке дашборда и после создания нового чата.

**Куда стучать:** `GET /api/v1/chat/sessions`

**Что придёт в ответ:**
```json
{
  "sessions": [
    {
      "id":         "aaa111-...",
      "title":      "Настройка воронки продаж",
      "created_at": "2026-04-12T10:00:00Z",
      "updated_at": "2026-04-12T15:30:00Z"
    },
    {
      "id":         "bbb222-...",
      "title":      "Обсуждение сделки Tesla",
      "created_at": "2026-04-12T14:00:00Z",
      "updated_at": "2026-04-12T14:45:00Z"
    }
  ],
  "count": 2
}
```

> Список отсортирован по `updated_at` — самые свежие чаты сверху.

**В сайдбаре отрисуй:**
- Кнопку «+ Новый чат» сверху
- Под ней — список `sessions`, каждый пункт: `title` + `updated_at`
- Активный чат выделяй цветом
- При клике на чат → загружай его историю (п. 5.3)

---

### 5.2. Создать новый чат

Пользователь нажал «+ Новый чат».

**Куда стучать:** `POST /api/v1/chat/sessions`

**Что отправить:**
```json
{}
```
*(Пустое тело — заголовок сгенерируется автоматически после первого сообщения)*

**Что придёт в ответ (статус `201 Created`):**
```json
{
  "session_id": "ccc333-...",
  "status":     "created"
}
```

**Что сделать после:**
1. Сохрани `session_id` как активную сессию
2. Загрузи историю нового чата (`GET /chat/history`) — там уже будет приветствие от Альбы
3. Подключи WebSocket к новому `session_id`
4. Обнови список чатов в сайдбаре (`GET /chat/sessions`)

---

### 5.3. Открыть существующий чат (загрузить историю)

Пользователь кликнул на чат в сайдбаре, или это произошло при первой загрузке.

**Куда стучать:** `GET /api/v1/chat/history?session_id=ccc333-...&limit=50`

**Параметры:**
- `session_id` (обязательно) — UUID сессии
- `limit` (опционально) — сколько сообщений загрузить (по умолчанию 50)

**Что придёт в ответ:**
```json
{
  "session_id": "ccc333-...",
  "messages": [
    {
      "id":         "msg-001",
      "session_id": "ccc333-...",
      "role":       "assistant",
      "content":    "Привет, Иван! Я Альба, твой AI-помощник в CRM. Чем могу помочь?",
      "created_at": "2026-04-12T10:00:00Z"
    },
    {
      "id":         "msg-002",
      "session_id": "ccc333-...",
      "role":       "user",
      "content":    "Создай сделку с Tesla на 500 тысяч",
      "created_at": "2026-04-12T10:01:00Z"
    },
    {
      "id":         "msg-003",
      "session_id": "ccc333-...",
      "role":       "assistant",
      "content":    "Готово! Сделка «Tesla» создана на сумму 500 000 ₽.",
      "created_at": "2026-04-12T10:01:05Z"
    }
  ],
  "count": 3
}
```

**Поле `role`:**
- `"user"` — сообщение пользователя (рендерь справа, с цветом пользователя)
- `"assistant"` — сообщение Альбы (рендерь слева, с цветом ИИ)

**Алгоритм при клике на чат:**
```
1. Поменять активную сессию в стейте
2. GET /chat/history?session_id=ВЫБРАННЫЙ_ID
3. Отрисовать все сообщения
4. Закрыть старый WebSocket
5. Открыть новый WebSocket с session_id=ВЫБРАННЫЙ_ID
6. Проскроллить чат вниз
```

---

### 5.4. Отправить сообщение

Пользователь написал текст и нажал Enter или кнопку отправки.

**Куда стучать:** `POST /api/v1/chat/message`

**Что отправить:**
```json
{
  "session_id": "ccc333-...",
  "message":    "Создай сделку с Tesla на 500 тысяч"
}
```

> `user_id` передавать НЕ нужно. Сервер сам берёт его из JWT-токена.

**Что придёт в ответ (статус `200 OK`):**
```json
{
  "type":       "message",
  "message_id": "msg-003",
  "session_id": "ccc333-...",
  "text":       "Готово! Сделка «Tesla» создана на сумму 500 000 ₽.",
  "intent":     "create_deal",
  "widget": {
    "type": "deal_draft",
    "data": {
      "id":     "deal-uuid-...",
      "name":   "Tesla",
      "amount": 500000,
      "stage":  "Lead"
    }
  },
  "created_at": "2026-04-12T10:01:05Z"
}
```

**Полный алгоритм отправки сообщения:**

```
1. Пользователь нажал Enter
2. Добавь сообщение пользователя в список (role: "user") ← сразу, не ждёшь ответа
3. Очисти input
4. Покажи индикатор "Альба печатает..."
5. Отправь POST /chat/message
6. Параллельно слушай WebSocket — там пойдёт стриминг текста
7. Когда придёт финальный ответ — покажи его (подробнее в разделе 6)
```

---

### 5.5. Поле `intent` — что сделал ИИ

Каждое сообщение от Альбы имеет поле `intent`. Оно говорит, **какое CRM-действие было выполнено**.

| `intent` | Что произошло | Типичная команда пользователя |
|---|---|---|
| `""` (пусто) | Обычный разговор, ничего не создано | «Привет», «Как дела?» |
| `create_deal` | Создана новая сделка | «Создай сделку с Tesla на 500к» |
| `update_deal_stage` | Сделка переведена на другой этап | «Переведи Tesla в Demo» |
| `get_deals` | Показан список сделок | «Покажи все сделки» |
| `create_company` | Создана компания | «Добавь компанию Google» |
| `enrich_company` | Компания обогащена данными | «Расскажи подробнее о Tesla» |
| `create_task` | Создана задача/напоминание | «Напомни позвонить Ивану завтра» |
| `get_tasks` | Показан список задач | «Что у меня на сегодня?» |
| `get_pipeline_report` | Сформирован отчёт по воронке | «Покажи конверсию воронки» |
| `get_business_summary` | Сформирован бизнес-дайджест | «Как у нас дела за неделю?» |
| `complete_onboarding` | Онбординг завершён | Автоматически |

---

## 6. WebSocket

### 6.1. Как подключиться

```
URL: wss://your-api.com/ws/chat?session_id=UUID_СЕССИИ
```

> ⚠️ В текущей реализации аутентификация проходит через `session_id`. Токен не нужен.

**Код подключения (копируй):**

```typescript
let wsConnection: WebSocket | null = null;

function connectToChat(sessionId: string) {
  // Если было старое соединение — закрой его
  if (wsConnection) {
    wsConnection.close();
  }

  const wsUrl = `wss://your-api.com/ws/chat?session_id=${sessionId}`;
  wsConnection = new WebSocket(wsUrl);

  wsConnection.onopen = () => {
    console.log('WebSocket подключен к сессии:', sessionId);
  };

  wsConnection.onmessage = (event) => {
    const data = JSON.parse(event.data);
    handleWebSocketEvent(data);
  };

  wsConnection.onclose = () => {
    console.log('WebSocket отключен');
    // Автопереподключение через 3 секунды
    setTimeout(() => connectToChat(sessionId), 3000);
  };

  wsConnection.onerror = (err) => {
    console.error('WebSocket ошибка:', err);
  };
}
```

---

### 6.2. Какие события приходят через WebSocket

Через WebSocket приходят **4 типа** событий. Различай их по полю `type`.

#### ❶ `stream_start` — Альба начала отвечать

```json
{
  "type": "stream_start",
  "session_id": "ccc333-...",
  "message_id": "msg-003"
}
```

**Что делать:** Создай пустой «баббл» (пузырёк) сообщения от ассистента. Покажи анимацию «печатает...»

#### ❷ `stream_chunk` — Следующий кусок текста ответа

```json
{
  "type": "stream_chunk",
  "session_id": "ccc333-...",
  "message_id": "msg-003",
  "text": "Готово! Сделка "
}
```

Это приходит **много раз**. Каждый раз ты **дописываешь** `text` к существующему бабблу.

Пример потока:
```
chunk 1: "Готово! "
chunk 2: "Сделка "
chunk 3: "«Tesla» "
chunk 4: "создана "
chunk 5: "на сумму "
chunk 6: "500 000 ₽."
```

Результат в бабблe: `"Готово! Сделка «Tesla» создана на сумму 500 000 ₽."`

#### ❸ `message` — Финальный, полный ответ

```json
{
  "type":       "message",
  "message_id": "msg-003",
  "session_id": "ccc333-...",
  "text":       "Готово! Сделка «Tesla» создана на сумму 500 000 ₽.",
  "intent":     "create_deal",
  "widget": {
    "type": "deal_draft",
    "data": { "id": "...", "name": "Tesla", "amount": 500000, "stage": "Lead" }
  },
  "created_at": "2026-04-12T10:01:05Z"
}
```

**Что делать:**
1. **Замени** потоковый баббл на финальный текст
2. Если есть `widget` — **отрисуй виджет** под сообщением (подробнее в разделе 7)
3. Убери индикатор «печатает...»

#### ❹ `notification` — Push-уведомление (не сообщение чата!)

```json
{
  "type": "notification",
  "notification": {
    "type":      "deal.created",
    "title":     "Новая сделка",
    "body":      "Tesla добавлена в CRM",
    "data":      { "amount": 500000 },
    "timestamp": "2026-04-12T..."
  }
}
```

**Что делать:** Покажи тост-уведомление (маленькое всплывающее окошко в углу экрана). Подробнее в разделе 8.

---

### 6.3. Полный обработчик (копируй)

```typescript
// Глобальное состояние стриминга
let streamingText = '';  // накапливаемый текст
let isStreaming = false;

function handleWebSocketEvent(data: any) {
  switch (data.type) {

    case 'stream_start':
      // Альба начала отвечать — создаём пустой баббл
      isStreaming = true;
      streamingText = '';
      addMessageBubble({
        id: data.message_id,
        role: 'assistant',
        content: '',
        isStreaming: true,
      });
      break;

    case 'stream_chunk':
      // Пришёл кусок текста — дописываём
      streamingText += data.text;
      updateMessageBubble(data.message_id, streamingText);
      break;

    case 'message':
      // Финальный ответ — заменяем стриминговый баббл
      isStreaming = false;
      replaceMessageBubble(data.message_id, {
        id: data.message_id,
        role: 'assistant',
        content: data.text,
        widget: data.widget || null,
        intent: data.intent || '',
      });
      streamingText = '';

      // Отрисовать виджет если есть
      if (data.widget) {
        renderWidget(data.widget);
      }

      // Обновить список чатов (заголовок мог измениться)
      refreshSessionList();
      break;

    case 'notification':
      // Показать тост
      showToast({
        title: data.notification.title,
        body: data.notification.body,
        type: data.notification.type,
      });
      break;
  }
}
```

---

## 7. Виджеты

Виджет — это **интерактивная карточка** под сообщением Альбы. Приходит в поле `widget`.

**Структура:**
```json
{
  "type": "тип_виджета",
  "data": { ...данные... }
}
```

**Общий принцип рендера:**
```typescript
function renderWidget(widget: any) {
  switch (widget.type) {
    case 'deal_draft':      return <DealCard data={widget.data} />;
    case 'task':            return <TaskCard data={widget.data} />;
    case 'tasks_list':      return <TasksList data={widget.data} />;
    case 'pipeline_report': return <PipelineChart data={widget.data} />;
    case 'business_summary':return <SummaryDashboard data={widget.data} />;
    case 'company_enriched':return <CompanyCard data={widget.data} />;
    case 'onboarding_result':return <OnboardingResult data={widget.data} />;
    case 'error':           return <ErrorBanner data={widget.data} />;
    default:                return null; // Неизвестный виджет — не показывай
  }
}
```

---

### 7.1. `deal_draft` — Карточка сделки

**Когда появляется:** intent = `create_deal`

```json
{
  "type": "deal_draft",
  "data": {
    "id":          "deal-uuid",
    "name":        "Tesla Motors",
    "amount":      500000,
    "stage":       "Lead",
    "description": "Контракт на поставку ПО",
    "created_at":  "2026-04-12T..."
  }
}
```

**Как отрисовать:**
```
┌─────────────────────────────────┐
│  🤝  СДЕЛКА                     │
│                                  │
│  Tesla Motors                    │
│  Сумма: 500 000 ₽               │
│  Этап: Lead                     │
│  Описание: Контракт на пост...  │
│                                  │
│  [Открыть]   [Изменить этап]    │
└─────────────────────────────────┘
```

---

### 7.2. `task` — Одна задача

**Когда появляется:** intent = `create_task`

```json
{
  "type": "task",
  "data": {
    "id":          "task-uuid",
    "title":       "Позвонить Ивану из Tesla",
    "description": "",
    "priority":    "high",
    "status":      "pending",
    "due_at":      "2026-04-13T10:00:00Z",
    "entity_type": "deal",
    "entity_id":   "deal-uuid"
  }
}
```

**Как отрисовать:**
```
┌─────────────────────────────────┐
│  ☐  Позвонить Ивану из Tesla    │
│     🔴 Высокий приоритет         │
│     📅 13 апреля, 10:00          │
│     🔗 Связана с: Tesla (сделка) │
└─────────────────────────────────┘
```

**Цвет приоритета:**
- `urgent` = 🔴 красный
- `high` = 🟠 оранжевый
- `medium` = 🟡 жёлтый
- `low` = ⚪ серый

---

### 7.3. `tasks_list` — Список задач (массив)

**Когда появляется:** intent = `get_tasks`

```json
{
  "type": "tasks_list",
  "data": [
    { "id": "t1", "title": "Позвонить Tesla", "priority": "high", "status": "pending", "due_at": "2026-04-13T10:00:00Z" },
    { "id": "t2", "title": "Подготовить КП",  "priority": "urgent", "status": "pending", "due_at": "2026-04-14T..." },
    { "id": "t3", "title": "Обновить данные",  "priority": "low", "status": "completed", "due_at": null }
  ]
}
```

**Как отрисовать:** Компактный список с чекбоксами. Завершённые задачи — зачёркнутый текст.

---

### 7.4. `pipeline_report` — Отчёт по воронке

**Когда появляется:** intent = `get_pipeline_report`

```json
{
  "type": "pipeline_report",
  "data": {
    "pipeline_name":       "Основная воронка",
    "total_deals":         42,
    "total_value":         2500000,
    "conversion_rate_pct": 14.3,
    "stages": [
      { "stage_name": "Lead",     "deal_count": 20, "total_value": 1000000, "avg_value": 50000 },
      { "stage_name": "Demo",     "deal_count": 12, "total_value": 800000,  "avg_value": 66667 },
      { "stage_name": "Proposal", "deal_count": 4,  "total_value": 400000,  "avg_value": 100000 },
      { "stage_name": "Won",      "deal_count": 6,  "total_value": 300000,  "avg_value": 50000 }
    ]
  }
}
```

**Как отрисовать:** Горизонтальная Bar Chart или Funnel-диаграмма. Для графиков используй библиотеку Chart.js или Recharts.

```
┌──────────────────────────────────────────────┐
│  📊  ВОРОНКА: Основная воронка               │
│                                               │
│  Всего сделок: 42 | На сумму: 2 500 000 ₽   │
│  Конверсия: 14.3%                            │
│                                               │
│  Lead     ████████████████████  20 (1 000 000)│
│  Demo     ████████████          12 (800 000)  │
│  Proposal ████                   4 (400 000)  │
│  Won      ██████                 6 (300 000)  │
└──────────────────────────────────────────────┘
```

---

### 7.5. `business_summary` — Бизнес-дайджест

**Когда появляется:** intent = `get_business_summary`

```json
{
  "type": "business_summary",
  "data": {
    "period":              "за последнюю неделю",
    "new_deals":           8,
    "new_deals_value":     400000,
    "deals_won":           2,
    "deals_won_value":     150000,
    "new_companies":       5,
    "tasks_created":       12,
    "tasks_completed":     9,
    "tasks_overdue":       1,
    "total_deals_active":  42,
    "total_companies":     87
  }
}
```

**Как отрисовать:** Сетка KPI-карточек:

```
┌──────────────────────────────────────────────────────────────┐
│  📈  ИТОГИ: за последнюю неделю                              │
│                                                              │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │ Новые сделки│  │ Закрыто     │  │ Компании    │         │
│  │     8       │  │     2       │  │     5       │         │
│  │   400 000 ₽ │  │   150 000 ₽ │  │   новых     │         │
│  └─────────────┘  └─────────────┘  └─────────────┘         │
│                                                              │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │ Задачи      │  │ Выполнено   │  │ ⚠️ Просрочено│         │
│  │    12       │  │     9       │  │     1       │         │
│  │   создано   │  │   из 12     │  │  СРОЧНО!    │         │
│  └─────────────┘  └─────────────┘  └──🔴─────────┘         │
└──────────────────────────────────────────────────────────────┘
```

> Если `tasks_overdue > 0` — выделяй красным!

---

### 7.6. `company_enriched` — Обогащённая компания

**Когда появляется:** intent = `enrich_company`

```json
{
  "type": "company_enriched",
  "data": {
    "company_name": "Tesla Motors",
    "industry":     "Automotive / Clean Energy",
    "website":      "https://tesla.com",
    "size":         "1000+",
    "description":  "American electric vehicle and clean energy company"
  }
}
```

---

### 7.7. `onboarding_result` — Результат онбординга

**Когда появляется:** intent = `complete_onboarding`

```json
{
  "type": "onboarding_result",
  "data": {
    "industry":    "b2b_saas",
    "pipeline":    "Основная воронка",
    "stages":      ["Lead", "Demo", "Proposal", "Won"],
    "departments": ["Sales", "Customer Success"]
  }
}
```

> ⚠️ Когда получишь этот виджет → подожди 2 секунды → запроси `/workspace/status` → если `onboarding_completed = true` → переключи UI на дашборд.

---

## 8. Уведомления

Уведомления приходят через тот же WebSocket, но с `type: "notification"`. Они **не связаны** с конкретным чатом — это события всего воркспейса.

**Какие уведомления бывают:**

| `notification.type` | `title` | `body` (пример) |
|---|---|---|
| `deal.created` | Новая сделка | Tesla добавлена в CRM |
| `deal.stage_changed` | Сделка обновлена | Tesla перешла в «Demo» |
| `task.created` | Задача создана | Позвонить Ивану |
| `task.overdue` | ⚠️ Задача просрочена | Подготовить КП (срок вчера) |
| `company.created` | Новая компания | Google добавлена в базу |

**Как показывать:**
Используй тост-уведомления (всплывашки) в правом верхнем углу. Они появляются на 5 секунд и исчезают. Для `task.overdue` — красный фон.

---

## 9. Обработка ошибок

Все ошибки приходят в формате:
```json
{
  "error": "Текст ошибки"
}
```

| Статус | Что случилось | Что делать |
|---|---|---|
| `400 Bad Request` | Ты отправил невалидные данные | Покажи текст из `error` |
| `401 Unauthorized` | Токен протух или не передан | Вызови `/auth/refresh` и повтори запрос |
| `403 Forbidden` | Нет прав на действие | Покажи «Нет доступа» |
| `409 Conflict` | Дублирование (email уже есть) | Покажи текст из `error` |
| `500 Internal Server` | Ошибка на сервере | Покажи «Попробуйте позже» |

---

## 10. Структура приложения

```
src/
├── api/
│   └── client.ts              ← Класс ApiClient из раздела 3.3
│
├── stores/  (или context/)
│   ├── authStore.ts            ← access_token, user, workspace
│   ├── chatStore.ts            ← sessions[], activeSessionId, messages{}
│   └── notificationStore.ts    ← очередь тостов
│
├── services/
│   └── websocket.ts            ← Класс подключения к WS из раздела 6
│
├── pages/
│   ├── LoginPage.tsx
│   ├── RegisterPage.tsx
│   ├── OnboardingPage.tsx      ← Полноэкранный чат
│   └── DashboardPage.tsx       ← Сайдбар + чат + виджеты
│
├── components/
│   ├── chat/
│   │   ├── ChatWindow.tsx       ← Окно чата (сообщения + инпут)
│   │   ├── MessageBubble.tsx    ← Один баббл: user или assistant
│   │   ├── MessageInput.tsx     ← Поле ввода + кнопка отправки
│   │   └── StreamingIndicator.tsx ← "Альба печатает..."
│   │
│   ├── sidebar/
│   │   ├── Sidebar.tsx          ← Боковая панель
│   │   ├── SessionItem.tsx      ← Один пункт чата в списке
│   │   └── NewChatButton.tsx    ← Кнопка "+ Новый чат"
│   │
│   ├── widgets/
│   │   ├── DealCard.tsx         ← widget.type === "deal_draft"
│   │   ├── TaskCard.tsx         ← widget.type === "task"
│   │   ├── TasksList.tsx        ← widget.type === "tasks_list"
│   │   ├── PipelineChart.tsx    ← widget.type === "pipeline_report"
│   │   ├── SummaryDashboard.tsx ← widget.type === "business_summary"
│   │   ├── CompanyCard.tsx      ← widget.type === "company_enriched"
│   │   └── OnboardingResult.tsx ← widget.type === "onboarding_result"
│   │
│   └── notifications/
│       └── Toast.tsx            ← Всплывающее уведомление
│
└── App.tsx                      ← Роутинг: Login | Register | Onboarding | Dashboard
```

---

## 11. Чеклист — что делать по порядку

Делай строго в таком порядке. Каждый пункт — отдельная задача.

### Этап 1: Авторизация (без этого ничего не работает)
- [ ] Сделать экран **Регистрации** → `POST /auth/register`
- [ ] Сделать экран **Логина** → `POST /auth/login`
- [ ] Реализовать `ApiClient` с автообновлением токена через `/auth/refresh`
- [ ] Сохранять `access_token` в памяти, `refresh_token` в localStorage
- [ ] Реализовать **Logout** → `POST /auth/logout` + очистка localStorage

### Этап 2: Определение экрана после логина
- [ ] После логина запрашивать `GET /workspace/status`
- [ ] Если `onboarding_completed = false` → полноэкранный чат (онбординг)
- [ ] Если `onboarding_completed = true` → дашборд с сайдбаром

### Этап 3: Чат (ядро приложения)
- [ ] Сделать **сайдбар** со списком чатов ← `GET /chat/sessions`
- [ ] Кнопка **«+ Новый чат»** ← `POST /chat/sessions`
- [ ] При клике на чат → загрузить историю ← `GET /chat/history?session_id=...`
- [ ] Поле ввода + кнопка отправки → `POST /chat/message`
- [ ] Подключить **WebSocket** ← `wss://.../ws/chat?session_id=...`
- [ ] Обработать `stream_start`, `stream_chunk`, `message` (стриминг текста)

### Этап 4: Виджеты
- [ ] Рендер `deal_draft` — карточка сделки
- [ ] Рендер `task` — карточка задачи
- [ ] Рендер `tasks_list` — список задач
- [ ] Рендер `pipeline_report` — график воронки
- [ ] Рендер `business_summary` — KPI дашборд
- [ ] Рендер `company_enriched` — карточка компании
- [ ] Рендер `onboarding_result` — итог настройки

### Этап 5: Push-уведомления
- [ ] Обработать WS событие `type: "notification"`
- [ ] Реализовать тост-уведомления (всплывашки)

### Этап 6: Полировка
- [ ] Индикатор «Альба печатает...» при стриминге
- [ ] Автоскролл чата при новых сообщениях
- [ ] Обновлять список чатов после отправки первого сообщения (заголовок генерируется автоматически)
- [ ] При `complete_onboarding` → автоматический переход на дашборд
