# 🚀 Frontend Developer's Guide: Alba CRM Interaction

This guide explains how to integrate the frontend with the Alba AI CRM backend, specifically focusing on the new **Onboarding Flow** and **AI Chat** features.

---

## 1. Authentication & Session Management

### Login / Registration
- **Register**: `POST /api/v1/auth/register`
- **Login**: `POST /api/v1/auth/login`
- **Refresh**: `POST /api/v1/auth/refresh` (use when Access Token expires in 15m)

**Important**: All authenticated requests must include the `Authorization: Bearer <access_token>` header.

---

## 2. Onboarding Workflow (Usability Flow)

When a user enters the app, the frontend should determine their state:

1. **Check Status**: Call `GET /api/v1/workspace/status`.
2. **Handle State**:
   - If `onboarding_completed: false`: Redirect to the **Onboarding Chat View**.
   - If `onboarding_completed: true`: Show the **Main CRM Dashboard**.

### Proactive Welcome
Upon registration, the backend automatically creates a chat session and generates a welcome message. 
- **Action**: Fetch the latest sessions using `GET /api/v1/chat/sessions` and load the history of the first one. Alba will already be there greeting the user.

---

## 3. The AI Chat API

### Sending Messages
`POST /api/v1/chat/message`
Request body:
```json
{
  "session_id": "uuid-here",
  "message": "We are a SaaS company selling AI tools."
}
```

### Handling the Response
The response is structured to support rich UI:
```json
{
  "text": "Great! I've set up your SaaS sales funnel...",
  "intent": "complete_onboarding",
  "widget": {
    "type": "onboarding_result",
    "data": {
      "stages": ["Lead", "Demo", "Trial", "Won"],
      "departments": ["Sales", "Product"]
    }
  }
}
```

### Widgets to Implement:
- **`deal`**: Show a deal card with confirm/cancel buttons.
- **`onboarding_result`**: Show a summary of created stages and departments.
- **`error`**: Show a toast or error alert.

---

## 4. Real-time Streaming (UX)

For a "ChatGPT-like" typing effect, connect to the WebSocket:
`GET /ws/chat?session_id=...`

**Events to listen for:**
1. `stream_start`: Prepare a new message bubble.
2. `stream_chunk`: Append text to the current bubble.
3. `message`: (Final event) Contains the full text, metadata, and optional **widget**.

---

## 5. Completing Onboarding
Once the AI has enough info, it will call the `complete_onboarding` tool internally. 
- The user will see a final message and a summary widget.
- After this, `GET /api/v1/workspace/status` will return `true`. 
- **Frontend Action**: Refresh the app state to switch from "Onboarding Mode" to "Dashboard Mode".

---

## Useful Endpoints Summary
| Method | Path | Description |
| :--- | :--- | :--- |
| `GET` | `/api/v1/workspace/status` | Current shop onboarding status |
| `GET` | `/api/v1/chat/history` | Load messages for a session |
| `GET` | `/api/v1/chat/sessions` | List of chat topics for the user |
| `POST` | `/api/v1/commands/commit` | Used to confirm "Draft" actions (like creating a user) |
