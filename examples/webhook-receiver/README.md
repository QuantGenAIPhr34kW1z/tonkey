# Webhook Receiver Example

A Node.js application demonstrating how to receive and verify tonkey webhooks.

## Features

- HMAC signature verification
- Event type handling
- Request logging
- Express.js server

## Quick Start

```bash
# Install dependencies
npm install

# Start server
npm start

# Or with environment variables
WEBHOOK_SECRET=your-secret PORT=3000 npm start
```

## Registering the Webhook

Use the tonkey API to register your webhook endpoint:

```bash
curl -X POST http://localhost:8080/v1/webhooks \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "http://your-server:3000/webhook",
    "event_type": "transaction",
    "secret": "your-webhook-secret"
  }'
```

## Webhook Payload

### Transaction Event

```json
{
  "event_type": "transaction",
  "timestamp": 1703001234,
  "data": {
    "id": "abc123",
    "from": "EQFrom...",
    "to": "EQTo...",
    "amount": 1000000000,
    "status": "confirmed"
  }
}
```

### Balance Change Event

```json
{
  "event_type": "balance_change",
  "timestamp": 1703001234,
  "data": {
    "address": "EQAddress...",
    "old_balance": 1000000000,
    "new_balance": 2000000000,
    "change": 1000000000
  }
}
```

## Signature Verification

tonkey signs webhooks using HMAC SHA-256. The signature is included in the `X-Tonkey-Signature` header.

```javascript
const crypto = require('crypto');

function verifySignature(payload, signature, secret) {
  const expected = crypto
    .createHmac('sha256', secret)
    .update(payload)
    .digest('hex');
  return crypto.timingSafeEqual(
    Buffer.from(signature),
    Buffer.from(expected)
  );
}
```
