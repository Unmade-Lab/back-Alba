# 🚀 Frontend Developer's Guide: Alba CRM API Reference

This document provides a comprehensive reference for the Alba AI CRM API, including all JSON structures, WebSocket events, and interaction patterns.

---

## 1. Authentication (JWT + Refresh Tokens)

### 1.1 Register Workspace
`POST /api/v1/auth/register`
**Request:**
```json
{
  "workspace_name": "Acme Corp",
  "user_name": "Admin",
  "email": "admin@acme.com",
  "password": "secret_password"
}
```
**Response (201 Created):**
```json
{
  "access_token": "eyJhbG...",
  "refresh_token": "8f8e...",
  "user": {
    "id": "uuid",
    "name": "Admin",
    "email": "admin@acme.com",
    "role": "admin",
    "workspace_id": "uuid"
  },
  "workspace": {
    "id": "uuid",
    "name": "Acme Corp"
  }
}
```

### 1.2 Login
`POST /api/v1/auth/login` (Same response as Register)

### 1.3 Refresh Token
`POST /api/v1/auth/refresh`
**Request:**
```json
{ "refresh_token": "token-from-cookies-or-localstorage" }
```
**Response (200 OK):**
```json
{
  "access_token": "new-access-token",
  "refresh_token": "new-rotated-refresh-token"
}
```

---

## 2. Onboarding & Workspace State

### 2.1 Get Workspace Status
`GET /api/v1/workspace/status`
**Response:**
```json
{
  "id": "uuid",
  "name": "Acme Corp",
  "onboarding_completed": false
}
```
*Use `onboarding_completed` to decide whether to show the "AI Setup Chat" or the "Dashboard".*

---

## 3. AI Chat & Messaging

### 3.1 Send Message
`POST /api/v1/chat/message`
**Request:**
```json
{
  "session_id": "uuid",
  "message": "We sell solar panels in Spain."
}
```
**Response:**
```json
{
  "id": "uuid",
  "role": "assistant",
  "content": "Perfect! I'll set up a Sales Funnel for solar panels.",
  "intent": "complete_onboarding",
  "widget": {
    "type": "onboarding_result",
    "data": {
      "industry": "services",
      "stages": ["Lead", "Proposal", "Won"],
      "departments": ["Sales", "Installation"]
    }
  },
  "created_at": "2026-04-12T..."
}
```

### 3.2 List Sessions
`GET /api/v1/chat/sessions`
**Response:**
```json
[
  {
    "id": "uuid",
    "title": "Solar Panel Onboarding",
    "created_at": "...",
    "updated_at": "..."
  }
]
```

---

## 4. WebSocket Streaming Protocol (UX)

Connect to: `ws://your-api.com/ws/chat?session_id=UUID&token=ACCESS_TOKEN`

### Event Types:

#### `stream_start` (Sent when AI begins generating text)
```json
{ "type": "stream_start" }
```

#### `stream_chunk` (Sent for every word/token)
```json
{ 
  "type": "stream_chunk", 
  "content": "Hello " 
}
```

#### `message` (Sent when the response is fully completed)
*Contains the full JSON message object as defined in section 3.1.*

---

## 5. UI Widgets Reference

When a message contains a `widget` field, render the corresponding UI component:

### `onboarding_result`
- **Data**: `industry` (string), `stages` (array), `departments` (array).
- **UI**: Success card showing the generated CRM structure.

### `deal`
- **Data**: `id`, `name`, `amount`, `stage`.
- **UI**: Interactive card with "Confirm" / "Edit" buttons. 
- **Action**: Confirmed deals should be sent to `POST /api/v1/commands/commit`.

### `error`
- **Data**: `message`.
- **UI**: Error toast or alert message.

---

## 6. Error Handling
Global error format (4xx/5xx):
```json
{
  "error": "Detailed error message here"
}
```
