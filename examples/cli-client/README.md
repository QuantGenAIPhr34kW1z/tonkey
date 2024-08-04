# CLI Client Example

A Go command-line client demonstrating tonkey API integration.

## Features

- Authentication with JWT
- Wallet balance queries
- Transaction submission
- Transaction history
- Configuration file support

## Quick Start

```bash
# Build
go build -o tonkey-cli

# Login
./tonkey-cli login -u demo -p demo

# Get balance
./tonkey-cli balance EQDemoAddress

# Send transaction
./tonkey-cli send --from EQFrom --to EQTo --amount 1000000000

# Query transactions
./tonkey-cli history --limit 10
```

## Configuration

Create `~/.tonkey/config.yaml`:

```yaml
api_url: http://localhost:8080
token: your-jwt-token
```

Or use environment variables:

```bash
export TONKEY_API_URL=http://localhost:8080
export TONKEY_TOKEN=your-jwt-token
```

## Commands

| Command               | Description                 |
| --------------------- | --------------------------- |
| `login`             | Authenticate and save token |
| `balance <address>` | Get wallet balance          |
| `send`              | Send a transaction          |
| `tx <id>`           | Get transaction details     |
| `history`           | List transactions           |
| `webhooks`          | Manage webhooks             |
| `apikeys`           | Manage API keys             |
