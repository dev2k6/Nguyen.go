# Nguyen.go Backend Modules

Các module backend tích hợp sẵn trong Nguyen.go framework, cung cấp đầy đủ primitives để xây dựng bất kỳ loại web application nào.

## Mục lục

1. [Database](#database)
2. [Validator](#validator)
3. [Auth](#auth)
4. [CSRF](#csrf)
5. [Upload](#upload)
6. [WebSocket](#websocket)
7. [i18n](#i18n)
8. [Mail](#mail)
9. [Event Bus](#event-bus)
10. [Rate Limit](#rate-limit)
11. [Migration CLI](#migration-cli)
12. [Docker](#docker)

---

## Database

Package: `internal/database/`

### Cấu hình (nguyen.config.yml)

```yaml
database:
  driver: "postgres"
  dsn: "postgres://user:pass@localhost:5432/dbname?sslmode=disable"
  max_open_conn: 25
  max_idle_conn: 5
  max_lifetime: 300
  migrations: "migrations"
```

### Sử dụng programmatic

```go
app := nguyen.New(
    nguyen.WithDatabase(config.DatabaseConfig{
        Driver: "postgres",
        DSN:    "postgres://user:pass@localhost:5432/dbname?sslmode=disable",
    }),
)
```

### Query Builder

```go
db := app.DB()

// SELECT
rows, err := db.Table("posts").
    Select("id", "title", "content").
    Where("author_id = ?", authorID).
    Where("published = ?", true).
    OrderBy("created_at DESC").
    Limit(20).
    Offset(0).
    Get(ctx)

// INSERT
result, err := db.Table("posts").Insert(ctx, map[string]any{
    "title":     "Hello World",
    "content":   "My first post",
    "author_id": userID,
})

// INSERT RETURNING
var id int
db.Table("posts").InsertReturning(ctx, map[string]any{
    "title":   "Hello World",
    "content": "My first post",
}, "id").Scan(&id)

// UPDATE
db.Table("posts").
    Where("id = ?", postID).
    Update(ctx, map[string]any{"title": "Updated Title"})

// DELETE
db.Table("posts").
    Where("id = ?", postID).
    Delete(ctx)

// COUNT
count, err := db.Table("posts").
    Where("author_id = ?", authorID).
    Count(ctx)

// Single row
row := db.Table("users").
    Where("email = ?", email).
    First(ctx)
row.Scan(&user.ID, &user.Name, &user.Email)

// Transaction
tx, err := db.Begin(ctx)
tx.ExecContext(ctx, "UPDATE wallets SET balance = balance - $1 WHERE user_id = $2", amount, fromID)
tx.ExecContext(ctx, "UPDATE wallets SET balance = balance + $1 WHERE user_id = $2", amount, toID)
tx.Commit()
```

---

## Validator

Package: `internal/validator/`

### Rules có sẵn

| Rule | Mô tả | Ví dụ |
|------|--------|-------|
| `required` | Không được trống | `"required"` |
| `email` | Email hợp lệ | `"email"` |
| `min:n` | Tối thiểu n ký tự hoặc giá trị | `"min:3"` |
| `max:n` | Tối đa n ký tự hoặc giá trị | `"max:255"` |
| `regex:pattern` | Khớp regex | `"regex:^[A-Z]"` |
| `numeric` | Phải là số | `"numeric"` |
| `alpha` | Chỉ chữ cái | `"alpha"` |
| `alphanum` | Chữ cái + số | `"alphanum"` |
| `url` | URL hợp lệ | `"url"` |
| `in:a,b,c` | Nằm trong danh sách | `"in:active,inactive"` |
| `notin:a,b` | Không nằm trong danh sách | `"notin:admin,root"` |
| `len:n` | Đúng n ký tự | `"len:10"` |
| `between:a,b` | Giá trị trong khoảng | `"between:1,100"` |

### Validate struct (tag-based)

```go
type CreatePostRequest struct {
    Title    string `json:"title" validate:"required|min:3|max:255"`
    Content  string `json:"content" validate:"required|min:10"`
    Category string `json:"category" validate:"required|in:tech,life,news"`
    Email    string `json:"email" validate:"required|email"`
}

v := validator.New()
req := CreatePostRequest{Title: "Hi", Content: "short"}
errors := v.Validate(req)
// errors = {"title": [...], "content": [...]}
```

### Validate map

```go
errors := v.ValidateMap(data, map[string]string{
    "email":    "required|email",
    "password": "required|min:8",
    "age":      "required|between:18,120",
})
```

### Middleware (auto-validate request body)

```go
// Map-based rules
app.Post("/api/posts", validator.Middleware(map[string]string{
    "title":   "required|min:3",
    "content": "required|min:10",
}), createPostHandler)

// Struct-based
app.Post("/api/register", validator.MiddlewareFor(&RegisterRequest{}), registerHandler)

// Access validated body in handler
func handler(c *fiber.Ctx) error {
    body := c.Locals("validated_body")
    // ...
}
```

### Custom rules

```go
v := validator.New()
v.RegisterRule("phone_vn", func(field string, value interface{}, param string) string {
    str, _ := toString(value)
    if !regexp.MustCompile(`^(0[3|5|7|8|9])+([0-9]{8})$`).MatchString(str) {
        return fmt.Sprintf("%s must be a valid Vietnamese phone number", field)
    }
    return ""
})
```

---

## Auth

Package: `internal/auth/`

### Cấu hình

```yaml
auth:
  enabled: true
  jwt_secret: "your-secret-key"
  jwt_expiry: 24h
  refresh_expiry: 168h
  session_ttl: 24h
  bcrypt_cost: 12
  token_header: "Authorization"
  cookie_name: "nguyen_session"
  cookie_secure: true
  cookie_httponly: true
  oauth:
    google:
      client_id: "xxx"
      client_secret: "xxx"
      redirect_url: "http://localhost:3000/auth/google/callback"
      scopes: ["openid", "email", "profile"]
      auth_url: "https://accounts.google.com/o/oauth2/v2/auth"
      token_url: "https://oauth2.googleapis.com/token"
      user_info_url: "https://www.googleapis.com/oauth2/v2/userinfo"
```

### JWT

```go
auth := app.Auth()

// Hash password
hash, err := auth.HashPassword("user_password")

// Verify password
err := auth.VerifyPassword(hash, "user_password")

// Generate token
token, err := auth.GenerateToken(&auth.Claims{
    UserID: "user_123",
    Role:   "admin",
    Email:  "user@example.com",
})

// Generate refresh token
refreshToken, err := auth.GenerateRefreshToken(&auth.Claims{UserID: "user_123"})

// Validate token
claims, err := auth.ValidateToken(tokenString)

// Refresh token pair
newAccess, newRefresh, err := auth.RefreshToken(oldRefreshToken)
```

### Middleware

```go
// Require authentication
api.Get("/me", app.Auth().Required(), meHandler)

// Require specific role
api.Delete("/users/:id", app.Auth().Role("admin"), deleteUserHandler)

// Optional auth (sets user info if token present)
api.Get("/feed", app.Auth().Optional(), feedHandler)

// Access user info in handler
func meHandler(c *fiber.Ctx) error {
    userID := auth.GetUserID(c)
    role := auth.GetUserRole(c)
    claims := auth.GetClaims(c)
    // ...
}
```

### Session

```go
sessions := app.Auth().Sessions()

// Create session
session, err := sessions.Create("user_123", "admin", map[string]interface{}{
    "theme": "dark",
})

// Get session
session, err := sessions.Get(sessionID)

// Refresh session TTL
sessions.Refresh(sessionID)

// Delete session
sessions.Delete(sessionID)

// Delete all sessions for a user (logout everywhere)
sessions.DeleteByUser("user_123")
```

### OAuth2

```go
// Setup routes
app.Get("/auth/google", app.Auth().OAuthRedirectHandler("google"))
app.Get("/auth/google/callback", app.Auth().OAuthCallbackHandler("google", func(c *fiber.Ctx, user *auth.OAuthUser) error {
    // user.ID, user.Email, user.Name, user.Avatar, user.Provider
    token, _ := app.Auth().GenerateToken(&auth.Claims{
        UserID: user.ID,
        Email:  user.Email,
    })
    return c.JSON(fiber.Map{"token": token})
}))
```

---

## CSRF

Package: `internal/csrf/`

### Cấu hình

```yaml
csrf:
  enabled: true
  token_length: 32
  cookie_name: "_nguyen_csrf"
  header_name: "X-CSRF-Token"
  form_field: "_csrf"
  expiry: 12h
  secure: true
  same_site: "Lax"
  skip_paths:
    - "/api/webhooks"
    - "/_nguyen"
```

### Sử dụng

CSRF middleware tự động được mount khi enabled. Token được set vào cookie và cần gửi lại qua header hoặc form field.

```go
// Get token trong handler (để inject vào HTML form)
func formPage(c *fiber.Ctx) error {
    token := app.CSRF().Token(c)
    // render form with hidden input: <input type="hidden" name="_csrf" value="{{token}}">
}
```

Client-side JavaScript:
```javascript
const token = document.cookie.match(/_nguyen_csrf=([^;]+)/)?.[1];

fetch('/api/data', {
    method: 'POST',
    headers: {
        'X-CSRF-Token': token,
        'Content-Type': 'application/json'
    },
    body: JSON.stringify(data)
});
```

---

## Upload

Package: `internal/upload/`

### Cấu hình

```yaml
upload:
  enabled: true
  max_size: 10485760  # 10MB
  allowed_types:
    - "image/*"
    - "application/pdf"
    - ".doc"
    - ".docx"
  storage_type: "local"  # "local" hoặc "s3"
  local_dir: "uploads"
  base_url: "/uploads"
  # S3 config (khi storage_type: "s3")
  s3_bucket: "my-bucket"
  s3_region: "ap-southeast-1"
```

### Sử dụng

```go
uploader := app.Uploader()

// Mount built-in routes
uploader.Routes(app.Fiber().Group("/api"))
// POST /api/upload         — single file (field: "file")
// POST /api/upload/multiple — multiple files (field: "files", max 10)
// DELETE /api/upload?path=  — delete file

// Custom handler
app.Post("/api/avatar", uploader.HandleSingle("avatar"))
app.Post("/api/gallery", uploader.HandleMultiple("images", 5))

// Programmatic upload
result, err := uploader.Upload(ctx, fileHeader)
// result.Path, result.URL, result.Filename, result.Size, result.MimeType

// Delete
err := uploader.Delete(ctx, "2024/01/15/1705312345.jpg")
```

---

## WebSocket

Package: `internal/ws/`

### Cấu hình

```yaml
websocket:
  enabled: true
  path: "/ws"
  max_message_size: 524288  # 512KB
  ping_interval: 30
```

### Sử dụng

```go
hub := app.WSHub()

// Event handlers
hub.OnConnect(func(client *ws.Client) {
    log.Printf("Client connected: %s", client.ID)
})

hub.OnDisconnect(func(client *ws.Client) {
    log.Printf("Client disconnected: %s", client.ID)
})

hub.OnMessage(func(client *ws.Client, msg *ws.Message) {
    switch msg.Type {
    case "chat":
        hub.Broadcast(msg.Room, msg.Payload)
    case "notify":
        hub.SendTo(msg.To, msg.Payload)
    }
})

// Server-side actions
hub.BroadcastJSON("room_name", map[string]interface{}{"type": "update", "data": data})
hub.SendTo("client_id", []byte(`{"type":"ping"}`))
```

Client-side:
```javascript
const ws = new WebSocket('ws://localhost:3000/ws?id=user_123');

ws.send(JSON.stringify({type: "join", room: "chat"}));
ws.send(JSON.stringify({type: "chat", room: "chat", payload: {text: "Hello"}}));
ws.send(JSON.stringify({type: "leave", room: "chat"}));
```

---

## i18n

Package: `internal/i18n/`

### Cấu hình

```yaml
i18n:
  enabled: true
  default_locale: "vi"
  locales: ["vi", "en", "ja"]
  translations_dir: "locales"
  url_prefix: false
  cookie_name: "nguyen_locale"
  query_param: "lang"
```

### Translation files

`locales/vi.json`:
```json
{
  "common": {
    "welcome": "Xin chào %s",
    "save": "Lưu",
    "cancel": "Hủy"
  },
  "errors": {
    "not_found": "Không tìm thấy",
    "unauthorized": "Chưa đăng nhập"
  }
}
```

`locales/en.json`:
```json
{
  "common": {
    "welcome": "Hello %s",
    "save": "Save",
    "cancel": "Cancel"
  },
  "errors": {
    "not_found": "Not found",
    "unauthorized": "Unauthorized"
  }
}
```

### Sử dụng

```go
i := app.I18n()

// Translate
i.T("vi", "common.welcome", "Nguyên")  // "Xin chào Nguyên"
i.T("en", "errors.not_found")           // "Not found"

// In handler (locale auto-detected by middleware)
func handler(c *fiber.Ctx) error {
    locale := i.LocaleFromContext(c)
    msg := i.T(locale, "common.welcome", "User")
    return server.OK(c, fiber.Map{"message": msg})
}

// Locale detection priority:
// 1. URL prefix (/en/about) — if url_prefix: true
// 2. Query param (?lang=en)
// 3. Cookie (nguyen_locale)
// 4. Accept-Language header
// 5. Default locale
```

---

## Mail

Package: `internal/mail/`

### Cấu hình

```yaml
mail:
  enabled: true
  host: "smtp.gmail.com"
  port: 465
  username: "your@gmail.com"
  password: "app-password"
  from_name: "My App"
  from_address: "noreply@myapp.com"
  tls: true
  templates_dir: "templates/email"
```

### Sử dụng

```go
mailer := app.Mailer()

// Plain text
err := mailer.Send(&mail.Message{
    To:      []string{"user@example.com"},
    Subject: "Hello",
    Body:    "Welcome to our platform!",
})

// HTML
err := mailer.Send(&mail.Message{
    To:      []string{"user@example.com"},
    Subject: "Welcome!",
    HTML:    "<h1>Welcome!</h1><p>Thanks for signing up.</p>",
})

// Template-based
// File: templates/email/welcome.html
// Content: <h1>Hi {{.Name}}</h1><p>Your account is ready.</p>
err := mailer.SendTemplate(
    []string{"user@example.com"},
    "Welcome",
    "welcome",
    map[string]interface{}{"Name": "Nguyên"},
)

// With attachment
err := mailer.Send(&mail.Message{
    To:      []string{"user@example.com"},
    Subject: "Your Report",
    HTML:    "<p>Report attached.</p>",
    Attachments: []mail.Attachment{
        {Filename: "report.pdf", Data: pdfBytes, MimeType: "application/pdf"},
    },
})
```

---

## Event Bus

Package: `internal/event/`

### Sử dụng

```go
bus := app.Events()

// Register handlers
bus.On("user.registered", func(payload interface{}) {
    user := payload.(User)
    app.Mailer().SendTemplate([]string{user.Email}, "Welcome!", "welcome", user)
})

bus.On("post.published", func(payload interface{}) {
    post := payload.(Post)
    // Notify subscribers, update cache, etc.
})

// Emit synchronous (handlers run in order)
bus.Emit("user.registered", user)

// Emit async (handlers run in goroutines)
bus.EmitAsync("post.published", post)

// Remove all handlers
bus.Off("user.registered")

// Check
bus.HasListeners("user.registered") // true/false
bus.ListenerCount("user.registered") // int
```

---

## Rate Limit

Package: `internal/server/`

### Per-endpoint rate limiting

```go
import "github.com/dev2k6/Nguyen.go/internal/server"

// 5 requests per minute for login
app.Post("/api/login", server.RateLimit(5, time.Minute), loginHandler)

// 100 requests per hour for API
app.Post("/api/data", server.RateLimit(100, time.Hour), dataHandler)

// Custom key (rate limit by user instead of IP)
app.Post("/api/actions", server.RateLimitByKey(30, time.Minute, func(c *fiber.Ctx) string {
    return auth.GetUserID(c) + ":" + c.Path()
}), actionHandler)
```

---

## Migration CLI

### Commands

```bash
# Create a new migration
nguyen migrate create add_users_table

# Run all pending migrations
nguyen migrate up

# Rollback last migration
nguyen migrate down

# Rollback last 3 migrations
nguyen migrate down 3

# Show migration status
nguyen migrate status
```

### Migration files

```
migrations/
├── 20240115120000_add_users_table.up.sql
├── 20240115120000_add_users_table.down.sql
├── 20240116090000_add_posts_table.up.sql
└── 20240116090000_add_posts_table.down.sql
```

Example UP:
```sql
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    role VARCHAR(50) DEFAULT 'user',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_users_email ON users(email);
```

Example DOWN:
```sql
DROP TABLE IF EXISTS users;
```

---

## Docker

### Build & Run

```bash
# Development (app + postgres + redis)
docker compose up -d

# Production
docker build -t myapp .
docker run -p 3000:3000 \
  -e DATABASE_DSN="postgres://..." \
  -e JWT_SECRET="..." \
  myapp
```

### Services

- **app** — Nguyen.go application (port 3000)
- **db** — PostgreSQL 16 (port 5432)
- **redis** — Redis 7 (port 6379)

Multi-stage Dockerfile: Go 1.25 Alpine build → Alpine 3.20 runtime.

---

## Ví dụ tổng hợp

```go
package main

import (
    "time"

    "github.com/dev2k6/Nguyen.go/internal/auth"
    "github.com/dev2k6/Nguyen.go/internal/config"
    "github.com/dev2k6/Nguyen.go/internal/server"
    "github.com/dev2k6/Nguyen.go/internal/validator"
    nguyen "github.com/dev2k6/Nguyen.go/pkg/nguyen"
    "github.com/gofiber/fiber/v2"
)

func main() {
    app := nguyen.New(
        nguyen.WithPort(3000),
        nguyen.WithPages("pages"),
        nguyen.WithDatabase(config.DatabaseConfig{
            Driver: "postgres",
            DSN:    "postgres://user:pass@localhost:5432/myapp?sslmode=disable",
        }),
        nguyen.WithAuth(config.AuthConfig{
            Enabled:   true,
            JWTSecret: "my-secret",
        }),
        nguyen.WithUpload(config.UploadConfig{
            Enabled:      true,
            MaxSize:      5 * 1024 * 1024,
            AllowedTypes: []string{"image/*"},
            StorageType:  "local",
            LocalDir:     "uploads",
        }),
        nguyen.WithI18n(config.I18nConfig{
            Enabled:       true,
            DefaultLocale: "vi",
            Locales:       []string{"vi", "en"},
        }),
        nguyen.WithMail(config.MailConfig{
            Enabled:     true,
            Host:        "smtp.gmail.com",
            Port:        465,
            FromAddress: "app@example.com",
            TLS:         true,
        }),
        nguyen.WithSetup(func(f *fiber.App) {
            api := f.Group("/api")

            // Public routes
            api.Post("/register", registerHandler)
            api.Post("/login", server.RateLimit(5, time.Minute), loginHandler)

            // Protected routes
            protected := api.Group("", app.Auth().Required())
            protected.Get("/me", meHandler)
            protected.Post("/posts", validator.Middleware(map[string]string{
                "title":   "required|min:3",
                "content": "required|min:10",
            }), createPostHandler)

            // Admin routes
            admin := api.Group("/admin", app.Auth().Role("admin"))
            admin.Get("/users", listUsersHandler)
            admin.Delete("/users/:id", deleteUserHandler)
        }),
    )

    // Event handlers
    app.Events().On("user.registered", func(payload interface{}) {
        // Send welcome email, log analytics, etc.
    })

    app.Listen(":3000")
}
```
