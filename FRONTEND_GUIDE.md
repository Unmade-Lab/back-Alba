# Alba CRM — Frontend Integration Guide

> Полная документация для фронтенд-разработчика. Здесь описано всё: авторизация, чат, WebSocket, виджеты, уведомления и управление данными.

---

## Содержание

1. [Архитектура системы](#1-архитектура-системы)
2. [Аутентификация](#2-аутентификация)
3. [Состояние воркспейса (Onboarding vs Dashboard)](#3-состояние-воркспейса)
4. [AI Чат — HTTP API](#4-ai-чат--http-api)
5. [WebSocket — Стриминг и Уведомления](#5-websocket--стриминг-и-уведомления)
6. [Система Виджетов](#6-система-виджетов)
7. [CRM Данные — REST API](#7-crm-данные--rest-api)
8. [Обработка ошибок](#8-обработка-ошибок)
9. [Рекомендуемая архитектура фронтенда](#9-рекомендуемая-архитектура-фронтенда)

---

## 1. Архитектура системы

```
Пользователь
    │
    ├─── HTTP REST API (авторизация, данные, сообщения)
    │         Base URL: https://api.your-domain.com/api/v1
    │
    └─── WebSocket (стриминг текста + push-уведомления)
              URL: wss://api.your-domain.com/ws/chat
```

**Важные принципы:**
- Все запросы требуют заголовка `Authorization: Bearer <access_token>`
- `workspace_id` определяется из JWT-токена на сервере — передавать его вручную в запросах **не нужно**
- WS-соединение открывается **один раз** при входе пользователя и живёт весь сеанс

---

## 2. Аутентификация

### 2.1 Регистрация

`POST /api/v1/auth/register`

**Request:**
```json
{
  "workspace_name": "Acme Corp",
  "user_name":      "Иван Петров",
  "email":          "ivan@acme.com",
  "password":       "supersecret123"
}
```

**Response `201 Created`:**
```json
{
  "access_token":  "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "550e8400-e29b-41d4-a716-446655440000",
  "user": {
    "id":           "uuid",
    "name":         "Иван Петров",
    "email":        "ivan@acme.com",
    "role":         "admin",
    "workspace_id": "uuid"
  },
  "workspace": {
    "id":   "uuid",
    "name": "Acme Corp"
  }
}
```

> ⚡ **После регистрации**: Альба автоматически создаёт первую сессию чата и отправляет приветственное сообщение. Сразу перенаправляй пользователя в чат.

---

### 2.2 Вход

`POST /api/v1/auth/login`

**Request:**
```json
{
  "email":    "ivan@acme.com",
  "password": "supersecret123"
}
```

**Response `200 OK`:** *(такой же формат как Register)*

---

### 2.3 Обновление токена

`POST /api/v1/auth/refresh`

**Request:**
```json
{
  "refresh_token": "550e8400-e29b-41d4-a716-446655440000"
}
```

**Response `200 OK`:**
```json
{
  "access_token":  "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "новый-ротированный-refresh-token"
}
```

> **Важно**: Refresh Token **ротируется** при каждом обновлении. Всегда сохраняй новый.

---

### 2.4 Выход

`POST /api/v1/auth/logout`

**Request:**
```json
{
  "refresh_token": "..."
}
```

**Response `200 OK`:**
```json
{ "message": "logged out" }
```

---

### 2.5 Стратегия хранения токенов

```javascript
// Рекомендуется хранить access_token в памяти (не в localStorage!)
// refresh_token — в httpOnly cookie или localStorage

class TokenManager {
  private accessToken: string | null = null;

  setTokens(access: string, refresh: string) {
    this.accessToken = access;
    localStorage.setItem('refresh_token', refresh);
  }

  getAccessToken() { return this.accessToken; }

  async refresh() {
    const rt = localStorage.getItem('refresh_token');
    const res = await fetch('/api/v1/auth/refresh', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: rt })
    });
    const data = await res.json();
    this.setTokens(data.access_token, data.refresh_token);
    return data.access_token;
  }
}
```

---

## 3. Состояние воркспейса

Сразу после входа/регистрации запрашивай состояние воркспейса. От него зависит, какой экран показывать.

`GET /api/v1/workspace/status`

**Response:**
```json
{
  "id":                   "uuid",
  "name":                 "Acme Corp",
  "onboarding_completed": false
}
```

### Логика переключения экранов:

```javascript
async function checkWorkspaceState() {
  const status = await api.get('/workspace/status');

  if (!status.onboarding_completed) {
    showScreen('ONBOARDING_CHAT'); // Показать чат с Альбой для настройки
  } else {
    showScreen('CRM_DASHBOARD');   // Показать полноценный дашборд
  }
}
```

```
┌─────────────────────────────────────────────────┐
│  onboarding_completed = false                    │
│                                                  │
│  Экран: AI Setup Chat                           │
│  - Чат занимает весь экран                      │
│  - Альба задаёт вопросы о бизнесе               │
│  - После завершения onboarding_completed = true │
└─────────────────────────────────────────────────┘
                        ↓
┌─────────────────────────────────────────────────┐
│  onboarding_completed = true                     │
│                                                  │
│  Экран: CRM Dashboard                           │
│  - Sidebar с чатами слева/справа                │
│  - Основная область с данными                   │
│  - Чат доступен всегда сбоку                    │
└─────────────────────────────────────────────────┘
```

---

## 4. AI Чат — HTTP API

### 4.1 Создать новую сессию чата

`POST /api/v1/chat/sessions`

**Request:**
```json
{
  "title": "Обсуждение новых лидов"
}
```
*(title — необязателен. Альба сама сгенерирует заголовок после первого сообщения)*

**Response `201 Created`:**
```json
{
  "session_id": "uuid",
  "status":     "created"
}
```

> **Важно**: После создания сессии Альба автоматически отправит приветственное сообщение. Сделай `GET /chat/history?session_id=...` чтобы его получить.

---

### 4.2 Список всех сессий (сайдбар)

`GET /api/v1/chat/sessions`

**Response:**
```json
{
  "sessions": [
    {
      "id":         "uuid",
      "title":      "Настройка воронки продаж",
      "created_at": "2026-04-12T10:00:00Z",
      "updated_at": "2026-04-12T15:30:00Z"
    },
    {
      "id":         "uuid",
      "title":      "Обсуждение сделки Tesla",
      "created_at": "2026-04-12T14:00:00Z",
      "updated_at": "2026-04-12T14:45:00Z"
    }
  ],
  "count": 2
}
```

> Список отсортирован по `updated_at DESC` — самые свежие сверху.

---

### 4.3 История сообщений

`GET /api/v1/chat/history?session_id=UUID&limit=50`

**Response:**
```json
{
  "session_id": "uuid",
  "messages": [
    {
      "id":         "uuid",
      "session_id": "uuid",
      "role":       "assistant",
      "content":    "Привет, Иван! Я Альба. Давай настроим CRM для Acme Corp?",
      "widget":     null,
      "intent":     "",
      "created_at": "2026-04-12T10:00:00Z"
    },
    {
      "id":         "uuid",
      "role":       "user",
      "content":    "Мы продаём B2B SaaS в сфере логистики",
      "widget":     null,
      "created_at": "2026-04-12T10:01:00Z"
    }
  ],
  "count": 2
}
```

**Поле `role`**: `"user"` | `"assistant"`

---

### 4.4 Отправить сообщение

`POST /api/v1/chat/message`

**Request:**
```json
{
  "session_id": "uuid",
  "message":    "Создай сделку с Tesla на 500 тысяч"
}
```

**Response `200 OK`:**
```json
{
  "type":       "message",
  "message_id": "uuid",
  "session_id": "uuid",
  "text":       "Сделка с Tesla создана! Сумма: 500 000. Она добавлена на первый этап воронки.",
  "intent":     "create_deal",
  "widget": {
    "type": "deal_draft",
    "data": {
      "id":          "uuid",
      "name":        "Tesla",
      "amount":      500000,
      "stage":       "Lead",
      "description": "",
      "created_at":  "2026-04-12T..."
    }
  },
  "created_at": "2026-04-12T..."
}
```

**Поле `intent`** — что сделал ИИ:

| `intent` | Описание |
|---|---|
| `""` (пусто) | Обычный диалог, без CRM-действия |
| `create_deal` | Создана новая сделка |
| `update_deal_stage` | Изменена стадия сделки |
| `create_company` | Создана компания |
| `create_task` | Создана задача/напоминание |
| `get_tasks` | Возвращён список задач |
| `get_deals` | Возвращён список сделок |
| `get_pipeline_report` | Сформирован отчёт по воронке |
| `get_business_summary` | Сформирован бизнес-дайджест |
| `enrich_company` | Компания обогащена данными |
| `complete_onboarding` | Онбординг завершён |

---

## 5. WebSocket — Стриминг и Уведомления

### 5.1 Подключение

```
ws://your-domain.com/ws/chat?session_id=UUID&token=ACCESS_TOKEN
```

```javascript
function connectWebSocket(sessionId, accessToken) {
  const ws = new WebSocket(
    `wss://api.your-domain.com/ws/chat?session_id=${sessionId}&token=${accessToken}`
  );

  ws.onmessage = (event) => {
    const data = JSON.parse(event.data);
    handleWSMessage(data);
  };

  ws.onclose = () => {
    // Переподключение через 3 секунды
    setTimeout(() => connectWebSocket(sessionId, accessToken), 3000);
  };

  return ws;
}
```

---

### 5.2 Типы WebSocket-событий

Все события имеют поле `type`. Обрабатывать нужно каждый тип по-своему.

#### Тип `stream_start` — ИИ начал отвечать
```json
{ "type": "stream_start" }
```
*Показать индикатор «печатает...» или пустой баббл сообщения.*

#### Тип `stream_chunk` — Следующая часть текста
```json
{
  "type":    "stream_chunk",
  "content": "Сделка с Tesla "
}
```
*Дописывать `content` к текущему бабблу ответа.*

#### Тип `message` — Финальный ответ (полная структура)
```json
{
  "type":       "message",
  "message_id": "uuid",
  "session_id": "uuid",
  "text":       "Сделка с Tesla создана! Сумма: 500 000.",
  "intent":     "create_deal",
  "widget": { ... },
  "created_at": "2026-04-12T..."
}
```
> Когда приходит `message`, **заменяй** потоковый текст финальным и рендери виджет.

#### Тип `notification` — Push-уведомление (workspace-level)
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

**Типы уведомлений:**

| `notification.type` | Когда приходит |
|---|---|
| `deal.created` | Альба создала новую сделку |
| `deal.stage_changed` | Изменена стадия сделки |
| `task.created` | Создана задача |
| `task.overdue` | Задача просрочена |
| `company.created` | Создана новая компания |

---

### 5.3 Полный алгоритм обработки чата

```javascript
let streamingText = '';
let streamingMessageId = null;

function handleWSMessage(data) {
  switch (data.type) {
    case 'stream_start':
      // Создать пустой баббл для стриминга
      streamingText = '';
      streamingMessageId = `streaming-${Date.now()}`;
      appendMessage({ id: streamingMessageId, role: 'assistant', content: '', streaming: true });
      break;

    case 'stream_chunk':
      // Дописать текст в баббл
      streamingText += data.content;
      updateMessage(streamingMessageId, streamingText);
      break;

    case 'message':
      // Заменить стриминговый баббл финальным сообщением
      replaceMessage(streamingMessageId, {
        id:      data.message_id,
        role:    'assistant',
        content: data.text,
        widget:  data.widget,
        intent:  data.intent,
      });
      streamingText = '';
      streamingMessageId = null;

      // Если есть виджет — рендерить его под сообщением
      if (data.widget) {
        renderWidget(data.widget);
      }
      break;

    case 'notification':
      // Показать тост-уведомление
      showToast(data.notification.title, data.notification.body, data.notification.type);
      break;
  }
}
```

---

## 6. Система Виджетов

Виджет — это интерактивный UI-компонент, который рендерится под сообщением Альбы. Приходит в поле `widget` сообщения.

**Структура виджета:**
```json
{
  "type": "тип_виджета",
  "data": { ... }
}
```

---

### Виджет `deal_draft` — Карточка сделки

```json
{
  "type": "deal_draft",
  "data": {
    "id":          "uuid",
    "name":        "Tesla Motors",
    "amount":      500000,
    "stage":       "Lead",
    "pipeline_id": "uuid",
    "stage_id":    "uuid",
    "description": "Крупный контракт на поставку ПО",
    "created_at":  "2026-04-12T..."
  }
}
```
**UI**: Карточка с кнопками «Открыть» / «Изменить стадию».

---

### Виджет `task` — Задача/Напоминание

```json
{
  "type": "task",
  "data": {
    "id":          "uuid",
    "title":       "Позвонить Ивану из Tesla",
    "description": "",
    "priority":    "high",
    "status":      "pending",
    "due_at":      "2026-04-13T10:00:00Z",
    "entity_type": "deal",
    "entity_id":   "uuid",
    "created_at":  "2026-04-12T..."
  }
}
```
**UI**: Карточка задачи с чекбоксом и отображением дедлайна. Цвет приоритета: `urgent`=красный, `high`=оранжевый, `medium`=жёлтый, `low`=серый.

---

### Виджет `tasks_list` — Список задач

```json
{
  "type": "tasks_list",
  "data": [
    {
      "id":       "uuid",
      "title":    "Подготовить КП для Tesla",
      "priority": "urgent",
      "status":   "pending",
      "due_at":   "2026-04-13T..."
    }
  ]
}
```
**UI**: Компактный список задач с чекбоксами.

---

### Виджет `pipeline_report` — Отчёт по воронке

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
**UI**: Горизонтальная Bar Chart или Funnel-диаграмма. Conversion Rate выделить крупно.

---

### Виджет `business_summary` — Бизнес-дайджест

```json
{
  "type": "business_summary",
  "data": {
    "period":           "за последнюю неделю",
    "new_deals":        8,
    "new_deals_value":  400000,
    "deals_won":        2,
    "deals_won_value":  150000,
    "new_companies":    5,
    "tasks_created":    12,
    "tasks_completed":  9,
    "tasks_overdue":    1,
    "total_deals_active": 42,
    "total_companies":  87
  }
}
```
**UI**: Дашборд с KPI-карточками. `tasks_overdue > 0` — выделить красным.

---

### Виджет `company_enriched` — Обогащённая компания

```json
{
  "type": "company_enriched",
  "data": {
    "company_name": "Tesla Motors",
    "industry":     "Automotive / Clean Energy",
    "website":      "https://tesla.com",
    "size":         "1000+",
    "description":  "American electric vehicle and clean energy company",
    "tags":         ["EV", "Technology", "Energy"]
  }
}
```
**UI**: Карточка с данными компании и кнопкой «Сохранить в CRM».

---

### Виджет `onboarding_result` — Результат онбординга

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
**UI**: Карточка-подтверждение с тем, что было настроено. После `complete_onboarding` — перенаправить на Dashboard.

---

### Виджет `error` — Ошибка

```json
{
  "type": "error",
  "data": {
    "message": "Не удалось создать сделку: компания не найдена"
  }
}
```
**UI**: Красный тост или inline-ошибка под сообщением.

---

## 7. CRM Данные — REST API

### 7.1 Статус воркспейса (для переключения экранов)

`GET /api/v1/workspace/status` — *описан в разделе 3*

---

### 7.2 Подтверждение черновика

После того как ИИ создал сделку/задачу как черновик, фронтенд может явно подтвердить её:

`POST /api/v1/commands/commit`

**Request:**
```json
{
  "session_id": "uuid",
  "intent":     "create_deal"
}
```

**Response `200 OK`:**
```json
{
  "status":  "committed",
  "message": "Сделка сохранена в CRM"
}
```

> Обычно ИИ сам сохраняет данные в БД при ответе. Этот эндпоинт нужен только для случаев, когда хочется двухшаговое подтверждение от пользователя.

---

## 8. Обработка ошибок

Все ошибки имеют единый формат:

```json
{
  "error": "Описание ошибки"
}
```

| HTTP код | Причина | Действие |
|---|---|---|
| `400` | Неверные данные запроса | Показать пользователю текст ошибки |
| `401` | Токен протух / не передан | Вызвать `refresh`, повторить запрос |
| `403` | Нет прав | Показать сообщение об ограничении |
| `500` | Ошибка сервера | Показать «Попробуйте позже» |

```javascript
// Пример interceptor для автообновления токена
async function apiRequest(url, options = {}) {
  let response = await fetch(url, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${tokenManager.getAccessToken()}`,
      ...options.headers,
    }
  });

  if (response.status === 401) {
    // Попробовать обновить токен и повторить запрос
    await tokenManager.refresh();
    response = await fetch(url, {
      ...options,
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${tokenManager.getAccessToken()}`,
      }
    });
  }

  if (!response.ok) {
    const err = await response.json();
    throw new Error(err.error || 'Unknown error');
  }

  return response.json();
}
```

---

## 9. Рекомендуемая архитектура фронтенда

### Структура экранов:

```
App
├── AuthScreen (если нет токена)
│   ├── LoginForm
│   └── RegisterForm
│
└── AppShell (если есть токен)
    ├── OnboardingChat (если !onboarding_completed)
    │   └── ChatWindow (full-screen)
    │
    └── CRMDashboard (если onboarding_completed)
        ├── Sidebar
        │   ├── NewChatButton → POST /chat/sessions
        │   └── ChatList → GET /chat/sessions (список с авто-заголовками)
        │
        ├── ChatPanel
        │   ├── MessageList
        │   │   └── Message (с виджетом если есть widget)
        │   └── InputBar → POST /chat/message
        │
        └── NotificationToaster ← WS type="notification"
```

### State Management (рекомендация):

```javascript
// Глобальное состояние
{
  auth: {
    user:          { id, name, email, role },
    workspace:     { id, name, onboarding_completed },
    accessToken:   string,
  },
  chat: {
    sessions:         [],          // список из GET /sessions
    activSessionId:   uuid,        // текущая открытая сессия
    messages:         {},          // { [sessionId]: Message[] }
    streamingText:    string,      // текст во время стриминга
    isStreaming:      boolean,
  },
  notifications: [],               // очередь уведомлений для тостов
}
```

### Жизненный цикл сессии чата:

```
1. Пользователь нажимает "Новый чат"
2. POST /chat/sessions → получаем session_id
3. Подключаем WS: ws://...?session_id=NEW_ID&token=...
4. GET /chat/history?session_id=NEW_ID → получаем приветствие от Альбы
5. Пользователь пишет сообщение → POST /chat/message
6. Получаем stream_start → показываем "печатает..."
7. Получаем stream_chunk × N → дорисовываем текст
8. Получаем message → показываем финальный ответ + виджет
9. Альба переименовывает сессию (обновится при следующем GET /sessions)
```

---

## Быстрый старт — Чеклист для разработчика

- [ ] Реализовать `AuthScreen` с Login/Register (разделы 2.1-2.2)
- [ ] Реализовать автообновление токена через `/auth/refresh` (раздел 2.3)
- [ ] После логина запрашивать `/workspace/status` и роутить на нужный экран (раздел 3)
- [ ] Реализовать создание сессий `POST /chat/sessions` (раздел 4.1)
- [ ] Реализовать сайдбар со списком `GET /chat/sessions` (раздел 4.2)
- [ ] Реализовать загрузку истории `GET /chat/history` (раздел 4.3)
- [ ] Реализовать отправку сообщения `POST /chat/message` (раздел 4.4)
- [ ] Подключить WebSocket и обработать `stream_start/chunk/message/notification` (раздел 5.3)
- [ ] Реализовать рендер всех типов виджетов (раздел 6)
- [ ] Добавить тост-уведомления для workspace-событий (раздел 5.2)
- [ ] Добавить interceptor для 401 → авто-refresh (раздел 8)
