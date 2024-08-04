<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/banner/banner.dark.svg">
    <source media="(prefers-color-scheme: light)" srcset="assets/banner/banner.light.svg">
    <img alt="RAVENGRID banner" src="assets/banner/banner.light.svg" width="900">
  </picture>
</p>

**Production-Ready TON Blockchain Gateway**

*Secure • Scalable • Simple*

[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![TON](https://img.shields.io/badge/TON-Blockchain-0088CC?logo=telegram&logoColor=white)](https://ton.org)

[Features](#-features) • [Quick Start](#-quick-start) • [Documentation](#-documentation) • [API Reference](#-api-reference)

</div>

---

## 🎯 Why tonkey?

Building on TON shouldn't be hard. **tonkey** is a production-ready backend gateway that handles all the complexity:

- ✅ **Battle-tested authentication** with JWT tokens and bcrypt password hashing
- ✅ **Real-time updates** via WebSockets for instant blockchain state changes
- ✅ **Event-driven webhooks** with HMAC signatures for secure notifications
- ✅ **Multi-tenant ready** with organization support and role-based access
- ✅ **Database flexibility** - SQLite for simplicity, PostgreSQL for scale
- ✅ **Production hardened** - rate limiting, metrics, structured logging, graceful shutdown

Unlike blockchain sdks that leave you to build everything else, tonkey gives you a **complete backend solution** out of the box.

---

## ✨ Key Features

<table>
<tr>
<td width="50%">

---

## 🚀 Quick Start

### Prerequisites

- **Go 1.23+** (with CGO for SQLite)
- **Make** (optional, for convenience)

### Install & Run in 60 Seconds

```bash
# 1. Clone the repository
git clone https://github.com/QuantGenAIPhr34kW1z/tonkey.git
cd tonkey

# 2. Build the binary
make build

# 3. Run with example config
make run
```

The server starts on `http://localhost:8080` 🎉

### Your First API Call

```bash
# Get a JWT token
TOKEN=$(curl -s -X POST http://localhost:8080/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"user":"demo","pass":"demo"}' | jq -r .token)

# Query a wallet balance
curl -s "http://localhost:8080/v1/wallet/EQDemoAddr/balance" \
  -H "Authorization: Bearer $TOKEN" | jq
```

**Output:**

```json
{
  "address": "EQDemoAddr",
  "balance": 1025000
}
```

---

## 🐳 Docker Quick Start

```bash
# Start with Docker Compose
docker compose up --build

# Or build and run manually
docker build -t tonkey .
docker run -p 8080:8080 -v $(pwd)/configs:/app/configs tonkey
```

---

## 📚 Core Concepts

### Provider Architecture

tonkey uses a **provider pattern** to abstract blockchain interactions. Swap providers without changing your application code:

```go
// Development: Use mock data
provider := ton.NewMock()

// Production: Connect to TON Center
provider := ton.NewTonCenterWithKey(
    "https://toncenter.com/api/v2",
    "your-api-key"
)
```

### Database Flexibility

Choose the right database for your needs:

**SQLite** - Perfect for small deployments

```yaml
database:
  driver: sqlite
  path: ./tonkey.db
```

**PostgreSQL** - Scales for high-traffic production

```yaml
database:
  driver: postgres
  connection_string: postgres://user:pass@localhost/tonkey?sslmode=require
```

### Real-Time Updates

Subscribe to blockchain events via WebSocket:

```javascript
const ws = new WebSocket('ws://localhost:8080/ws?token=' + jwtToken);

ws.send(JSON.stringify({
  action: 'subscribe',
  addresses: ['EQDx1Vg0K0jMKS2...']
}));

ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);
  if (msg.type === 'balance_update') {
    console.log('New balance:', msg.data.balance);
  }
};
```

---

## 🔌 API Reference

### Authentication

#### **POST** `/auth/login`

Get a JWT token for authenticated requests.

**Request:**

```json
{
  "user": "alice",
  "pass": "secure-password"
}
```

**Response:**

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

---

### Wallet Operations

#### **GET** `/v1/wallet/{address}/balance`

Query wallet balance.

**Headers:** `Authorization: Bearer <token>`

**Response:**

```json
{
  "address": "EQDx1Vg0K0jMKS2N-W5F0NzZPLGp0tPBtQmILqFmXOL_vXqp",
  "balance": 1025000
}
```

---

### Transaction Operations

#### **POST** `/v1/tx/send`

Submit a transaction to the blockchain.

**Headers:** `Authorization: Bearer <token>`

**Request:**

```json
{
  "from": "EQDx1Vg0K0jMKS2...",
  "to": "EQAnotherAddress...",
  "amount": 5000
}
```

**Response:**

```json
{
  "id": "a1b2c3d4e5f6..."
}
```

#### **GET** `/v1/tx/{id}`

Get transaction details and status.

**Response:**

```json
{
  "id": "a1b2c3d4e5f6...",
  "created_at": 1703001234,
  "from": "EQDx1...",
  "to": "EQAnother...",
  "amount": 5000,
  "status": "submitted"
}
```

---

### Query Endpoint

#### **POST** `/v1/query`

Query transactions with structured filters.

**Request:**

```json
{
  "entity": "tx",
  "filters": {
    "status": "submitted",
    "from": "EQDx1..."
  },
  "limit": 50
}
```

**Response:**

```json
{
  "entity": "tx",
  "count": 2,
  "rows": [...]
}
```

---

### WebSocket Endpoint

#### **WS** `/ws?token={jwt}`

Real-time updates via WebSocket.

**Subscribe to addresses:**

```json
{
  "action": "subscribe",
  "addresses": ["EQDx1Vg0K0jMKS2..."]
}
```

**Receive balance updates:**

```json
{
  "type": "balance_update",
  "address": "EQDx1...",
  "data": {
    "balance": 1025000,
    "time": 1703001234
  }
}
```

---

### Webhooks

#### **POST** `/v1/webhooks`

Create webhook subscription.

**Request:**

```json
{
  "url": "https://myapp.com/webhook",
  "event_type": "transaction",
  "filter_address": "EQDx1...",
  "secret": "webhook-secret-for-hmac"
}
```

---

### Metrics

#### **GET** `/metrics`

Prometheus metrics endpoint (when enabled).

---

## ⚙️ Configuration

Create `config.yaml`:

```yaml
# Server configuration
bind: "0.0.0.0:8080"
jwt_secret: "change-me-in-production"  # Generate with: openssl rand -base64 32

# Database - choose SQLite or PostgreSQL
database:
  driver: sqlite
  path: ./tonkey.db
  # driver: postgres
  # connection_string: postgres://user:pass@localhost/tonkey

# TON Provider
provider:
  kind: mock  # or "toncenter"
  toncenter_url: https://toncenter.com/api/v2
  api_key: your-api-key-here  # Optional, for higher rate limits

# Rate Limiting
rate_limit:
  enabled: true
  requests_per_second: 100
  burst: 200

# WebSocket Support
websocket:
  enabled: true

# Webhooks
webhooks:
  enabled: true
  workers: 10

# Metrics
metrics:
  enabled: true

# Telegram Bot (optional)
telegram:
  enabled: false
  token: "YOUR_BOT_TOKEN"
  allow_chats: [123456789]

# OpenAPI Documentation 
openapi:
  enabled: true
  path: /api/docs

# Admin Dashboard 
dashboard:
  enabled: true

# Distributed Tracing 
tracing:
  enabled: true
  service_name: tonkey
  exporter: stdout  # otlp, jaeger, zipkin, stdout
  sample_rate: 1.0

# Geographic Rate Limiting 
geo_rate_limit:
  enabled: true
  default_rps: 100
  default_burst: 200
  blocked_countries: []  # e.g., ["XX", "YY"]

# NFT Support 
nft:
  enabled: true
  max_collections: 100
  marketplace_enabled: true

# Jetton (Token) Support 
jetton:
  enabled: true
  max_tokens: 50
  allow_minting: true
  allow_burning: true

# GraphQL API 
graphql:
  enabled: true
  path: /graphql
  playground: true
```

---

## 🔒 Security Best Practices

Before deploying to production:

1. **Generate a strong JWT secret**: `openssl rand -base64 32`
2. **Use HTTPS** with a reverse proxy (nginx, Caddy)
3. **Enable rate limiting** to prevent abuse
4. **Use PostgreSQL** for multi-instance deployments
5. **Enable metrics** for monitoring
6. **Rotate secrets regularly**
7. **Use environment variables** for sensitive config
8. **Run security audits** before handling real funds

See [SECURITY.md](SECURITY.md) for detailed security guidance.

---

## 🧪 Development

### Project Structure

```
tonkey/
├── cmd/tonkeyd/           # Main application entry point
│   ├── main.go
│   └── schema.sql         # Embedded database schema
├── internal/              # Private application code
│   ├── api/              # HTTP handlers and routing
│   ├── apikeys/          # API key authentication
│   ├── audit/            # Audit logging
│   ├── auth/             # JWT token management
│   ├── batch/            # Transaction batching
│   ├── config/           # Configuration loading
│   ├── contracts/        # Smart contract interactions
│   ├── dashboard/        # Admin dashboard UI
│   ├── georatelimit/     # Geographic rate limiting
│   ├── graphql/          # GraphQL API endpoint
│   ├── ipfilter/         # IP allowlist/blocklist
│   ├── jetton/           # Jetton (token) support
│   ├── nft/              # NFT support
│   ├── jobs/             # Background job system
│   ├── logger/           # Structured logging
│   ├── metrics/          # Prometheus metrics
│   ├── middleware/       # Rate limiting, etc.
│   ├── migrations/       # Database migrations
│   ├── multisig/         # Multi-signature wallets
│   ├── openapi/          # OpenAPI/Swagger docs
│   ├── query/            # Query engine
│   ├── registration/     # User self-registration
│   ├── reset/            # Password reset
│   ├── store/            # Database abstraction (SQLite/PostgreSQL)
│   ├── tg/               # Telegram bot
│   ├── ton/              # TON provider abstraction
│   ├── totp/             # Two-factor authentication
│   ├── tracing/          # Distributed tracing
│   ├── users/            # User & organization management
│   ├── validator/        # Input validation
│   ├── webhooks/         # Webhook delivery system
│   └── websocket/        # WebSocket hub
├── examples/             # Example applications
│   ├── cli-client/       # Go CLI client
│   ├── react-wallet/     # React wallet frontend
│   └── webhook-receiver/ # Node.js webhook handler
├── configs/              # Configuration files
├── Dockerfile
├── docker-compose.yml
├── Makefile
└── README.md
```

### Running Tests

```bash
# Run all tests
make test

# With coverage report
make test-coverage

# Run specific package
go test -v ./internal/auth
```

### Code Quality

```bash
# Format code
make fmt

# Lint code
make lint

# Vet code
make vet

# Quick pre-commit check
make check
```

---

## 🎓 Telegram Bot

Enable the Telegram bot for wallet monitoring:

```yaml
telegram:
  enabled: true
  token: "YOUR_BOT_TOKEN_FROM_BOTFATHER"
  allow_chats: [123456789]
```

### Bot Commands

- `/balance <address>` - Query wallet balance
- `/watch <address>` - Add address to watchlist
- `/unwatch <address>` - Remove from watchlist
- `/watchlist` - Show all watched addresses

---

## 📊 Monitoring & Metrics

Enable Prometheus metrics:

```yaml
metrics:
  enabled: true
```

Access metrics at `http://localhost:8080/metrics`

**Key Metrics:**

- `tonkey_http_requests_total` - HTTP request count by endpoint and status
- `tonkey_http_request_duration_seconds` - Request latency histogram
- `tonkey_provider_calls_total` - TON provider API calls
- `tonkey_transactions_total` - Transaction count by status
- `tonkey_websocket_connections` - Active WebSocket connections
- `tonkey_webhook_deliveries_total` - Webhook delivery success/failure

Use with Grafana for beautiful dashboards 📈

---

## 🙏 Acknowledgments

Built with:

- [TON](https://ton.org) - The Open Network
- [go-telegram-bot-api](https://github.com/go-telegram-bot-api/telegram-bot-api) - Telegram Bot API wrapper
- [golang-jwt](https://github.com/golang-jwt/jwt) - JWT implementation
- [gorilla/websocket](https://github.com/gorilla/websocket) - WebSocket support
- [Prometheus](https://prometheus.io/) - Metrics and monitoring

---
<div align="center">

**Built with ❤️ for the TON ecosystem**
If tonkey helps you build on TON, give it a ⭐ to show your support!

[Get Started](#-quick-start) • [View Docs](#-documentation) • [Report Bug](https://github.com/QuantGenAIPhr34kW1z/tonkey/issues)

</div>
