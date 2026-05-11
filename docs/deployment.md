# Deployment

Nguyen.go applications can be deployed as a single binary, Docker container, or static files on a CDN.

## Single Binary

The simplest deployment — one executable serves everything.

### Build

```bash
# Build the application
nguyen build

# Compile the server binary
go build -o server ./cmd/nguyen/

# Or cross-compile for Linux
GOOS=linux GOARCH=amd64 go build -o server ./cmd/nguyen/
```

### Run

```bash
./server start --dir .nguyen --port 8080
```

### What You Need to Deploy

```
server              # The compiled binary
.nguyen/            # Build output (HTML, WASM, assets)
config/             # Configuration (optional, has defaults)
```

## Docker

### Minimal Dockerfile

```dockerfile
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o nguyen ./cmd/nguyen/
RUN ./nguyen build --no-wasm

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/nguyen .
COPY --from=builder /app/.nguyen .nguyen
COPY --from=builder /app/config config
EXPOSE 3000
CMD ["./nguyen", "start", "--port", "3000"]
```

### With TinyGo (WASM support)

```dockerfile
FROM tinygo/tinygo:0.34.0 AS wasm-builder
WORKDIR /app
COPY . .
RUN go build -o nguyen ./cmd/nguyen/
RUN ./nguyen build

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=wasm-builder /app/nguyen .
COPY --from=wasm-builder /app/.nguyen .nguyen
COPY --from=wasm-builder /app/config config
EXPOSE 3000
CMD ["./nguyen", "start", "--port", "3000"]
```

### Docker Compose

```yaml
version: '3.8'
services:
  web:
    build: .
    ports:
      - "3000:3000"
    environment:
      - PORT=3000
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "--spider", "http://localhost:3000/_nguyen/runtime"]
      interval: 30s
      timeout: 5s
      retries: 3
```

## Static Export (CDN)

For sites that don't need a server at runtime.

### Build

```bash
nguyen export --output dist
```

### Output Structure

```
dist/
├── index.html
├── about/index.html
├── blog/index.html
├── sitemap.xml
├── robots.txt
├── llms.txt
├── styles/
│   └── output.css
└── public/
    └── favicon.ico
```

### Deploy to Platforms

**Netlify:**
```toml
# netlify.toml
[build]
  command = "nguyen export --output dist"
  publish = "dist"
```

**Vercel:**
```json
{
  "buildCommand": "nguyen export --output dist",
  "outputDirectory": "dist"
}
```

**Cloudflare Pages:**
- Build command: `nguyen export --output dist`
- Build output directory: `dist`

**AWS S3 + CloudFront:**
```bash
nguyen export --output dist
aws s3 sync dist/ s3://my-bucket/ --delete
aws cloudfront create-invalidation --distribution-id XXXXX --paths "/*"
```

## Reverse Proxy

### Nginx

```nginx
server {
    listen 80;
    server_name example.com;

    location / {
        proxy_pass http://127.0.0.1:3000;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_cache_bypass $http_upgrade;

        # SSE support for HMR (dev only)
        proxy_buffering off;
        proxy_set_header X-Accel-Buffering no;
    }
}
```

### Caddy

```caddyfile
example.com {
    reverse_proxy localhost:3000
}
```

## Systemd Service

```ini
# /etc/systemd/system/nguyen-app.service
[Unit]
Description=Nguyen.go Application
After=network.target

[Service]
Type=simple
User=www-data
WorkingDirectory=/opt/my-app
ExecStart=/opt/my-app/server start --port 3000
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl enable nguyen-app
sudo systemctl start nguyen-app
```

## Environment Variables

Nguyen.go reads configuration from YAML, but you can override the port via CLI flags:

```bash
nguyen start --port $PORT
```

## Health Check

The production server exposes a runtime endpoint:

```
GET /_nguyen/runtime
```

Response:
```json
{
  "version": "1.0.0",
  "chunkMode": true
}
```

Use this for load balancer health checks.

## Performance Considerations

### Production Checklist

- [ ] Build with `--no-wasm` if you only need SSR (smaller deploy)
- [ ] Enable compression in your reverse proxy (Nguyen.go also compresses)
- [ ] Set up a CDN for static assets (`/styles/`, `/public/`, `/chunks/`)
- [ ] Use ISR mode for pages that change infrequently
- [ ] Enable PWA for offline support and faster repeat visits

### Built-in Optimizations

- Brotli/gzip compression (via Fiber middleware)
- ETag headers for cache validation
- Immutable cache headers for WASM chunks
- Rate limiting (600 req/min/IP)
- Security headers (HSTS, X-Frame-Options, CSP-adjacent)
- Panic recovery middleware
- Graceful shutdown on SIGTERM/SIGINT

### Recommended Architecture

```
                    ┌─────────────┐
                    │   CDN/Edge  │
                    │  (static)   │
                    └──────┬──────┘
                           │
┌──────────┐       ┌──────▼──────┐
│  Client  │◄─────►│   Nginx /   │
│ (Browser)│       │   Caddy     │
└──────────┘       └──────┬──────┘
                           │
                    ┌──────▼──────┐
                    │  Nguyen.go  │
                    │   Server    │
                    └─────────────┘
```

Static assets (CSS, images, WASM chunks) go through CDN. Dynamic requests (SSR, ISR, API) go to the Nguyen.go server.
