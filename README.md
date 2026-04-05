# Alba CRM — API Documentation

Документация для Frontend-разработчика по интеграции с бэкендом Alba CRM (ИИ-ассистент).

---

## 🔗 Базовый URL
В локальной разработке все запросы отправляются по адресу:
`http://localhost:8080`

Обязательно добавьте заголовок `Content-Type: application/json` ко всем POST-запросам.

---

## 1. Подключение к WebSockets (Events)

Вебсокет используется **только для получения** событий и ответов сервера в реальном времени. Мы **не отправляем** текст пользователя в WebSocket. Текст пользователя отправляется через HTTP POST (см. ниже).

**Метод:** `GET`
**URL:** `ws://localhost:8080/ws/chat`
**Параметры:**
- `session_id` (string, обязательный) — уникальный ID сессии чата.

**Пример подключения в JS:**
```javascript
const ws = new WebSocket('ws://localhost:8080/ws/chat?session_id=user123_session');

ws.onmessage = (event) => {
    const data = JSON.parse(event.data);
    console.log("Получено сообщение от ИИ:", data);
};
```

---

## 2. Отправка сообщения в Чат

Главный эндпоинт. Вы отправляете сообщение пользователя через этот POST запрос. Бэкенд ответит JSON-ом, а также **продублирует этот же JSON в канал WebSocket**, чтобы фронтенд мог мгновенно среагировать.

**Метод:** `POST`
**URL:** `/api/v1/chat/message`

**Body:**
```json
{
  "session_id": "user123_session",
  "message": "Привет! Создай сделку на 500$"
}
```

**Ответ (200 OK):**
```json
{
  "message_id": "f758808f-2d09-4f94-816f-9758771f915d",
  "session_id": "user123_session",
  "text": "Отлично, я готов создать новую сделку...",
  "intent": "create_deal",
  "widget": {
    "type": "create_deal",
    "data": {
       "name": "New Deal",
       "amount": 500
    }
  },
  "created_at": "2026-04-05T22:33:13.2041314+05:00"
}
```

*Примечание: Поле `widget` может отсутствовать (`null` или не включено), если ответ ИИ — это просто текст. Если `widget` пришел, фронтенд должен отрендерить кастомный UI компонент (например, форму сделки).*

---

## 3. Получение истории переписки

Метод для загрузки всей истории сообщений при открытии страницы чата.

**Метод:** `GET`
**URL:** `/api/v1/chat/history`
**Параметры (Query):**
- `session_id` (string, обязательный) — уникальный ID сессии.
- `limit` (number, опциональный) — количество последних сообщений (по умолчанию 50).

**Пример запроса:**
`GET /api/v1/chat/history?session_id=user123_session&limit=20`

**Ответ (200 OK):**
```json
{
  "session_id": "user123_session",
  "count": 2,
  "messages": [
    {
      "id": "aaa-...",
      "session_id": "user123_session",
      "user_id": null,
      "role": "user",
      "content": "Создай сделку",
      "created_at": "2026..."
    },
    {
      "id": "bbb-...",
      "session_id": "user123_session",
      "role": "assistant",
      "content": "Сделка создана!",
      "widget": "{\"type\": \"create_deal\", \"data\": {}}",
      "intent": "create_deal",
      "created_at": "2026..."
    }
  ]
}
```

---

## 4. Health Check
Проверка того, что сервер живой.

**Метод:** `GET`
**URL:** `/health`

**Ответ (200 OK):**
```json
{
  "status": "ok",
  "service": "alba-crm"
}
```