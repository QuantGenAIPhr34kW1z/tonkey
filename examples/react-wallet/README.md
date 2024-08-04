# React Wallet Example

A simple React application demonstrating tonkey API integration.

## Features

- User authentication with JWT
- Wallet balance display
- Transaction history
- Send transactions
- Real-time updates via WebSocket

## Quick Start

```bash
# Install dependencies
npm install

# Start development server
npm start
```

## Project Structure

```
react-wallet/
├── src/
│   ├── App.jsx           # Main application
│   ├── api/
│   │   └── tonkey.js     # API client
│   ├── components/
│   │   ├── Login.jsx
│   │   ├── Wallet.jsx
│   │   ├── SendForm.jsx
│   │   └── History.jsx
│   └── hooks/
│       └── useWebSocket.js
├── package.json
└── README.md
```

## API Integration

```javascript
import { TonkeyClient } from './api/tonkey';

const client = new TonkeyClient('http://localhost:8080');

// Login
const token = await client.login('username', 'password');

// Get balance
const balance = await client.getBalance('EQAddress...');

// Send transaction
const txId = await client.sendTransaction({
  from: 'EQFrom...',
  to: 'EQTo...',
  amount: 1000000000 // 1 TON in nanoTON
});
```

## Environment Variables

Create a `.env` file:

```
REACT_APP_API_URL=http://localhost:8080
```
