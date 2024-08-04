/**
 * tonkey Webhook Receiver Example
 *
 * This example demonstrates how to receive and verify webhooks from tonkey.
 */

const express = require('express');
const crypto = require('crypto');

const app = express();

// Configuration
const PORT = process.env.PORT || 3000;
const WEBHOOK_SECRET = process.env.WEBHOOK_SECRET || 'your-webhook-secret';

// Parse raw body for signature verification
app.use('/webhook', express.raw({ type: 'application/json' }));
app.use(express.json());

/**
 * Verify HMAC signature
 */
function verifySignature(payload, signature, secret) {
  if (!signature) {
    return false;
  }

  const expected = crypto
    .createHmac('sha256', secret)
    .update(payload)
    .digest('hex');

  try {
    return crypto.timingSafeEqual(
      Buffer.from(signature),
      Buffer.from(expected)
    );
  } catch {
    return false;
  }
}

/**
 * Webhook endpoint
 */
app.post('/webhook', (req, res) => {
  const signature = req.headers['x-tonkey-signature'];
  const payload = req.body;

  // Verify signature
  if (!verifySignature(payload, signature, WEBHOOK_SECRET)) {
    console.error('Invalid webhook signature');
    return res.status(401).json({ error: 'Invalid signature' });
  }

  // Parse payload
  let event;
  try {
    event = JSON.parse(payload.toString());
  } catch (e) {
    console.error('Invalid JSON payload:', e);
    return res.status(400).json({ error: 'Invalid JSON' });
  }

  console.log('Received webhook:', JSON.stringify(event, null, 2));

  // Handle different event types
  switch (event.event_type) {
    case 'transaction':
      handleTransactionEvent(event.data);
      break;
    case 'balance_change':
      handleBalanceChangeEvent(event.data);
      break;
    default:
      console.log('Unknown event type:', event.event_type);
  }

  // Acknowledge receipt
  res.status(200).json({ received: true });
});

/**
 * Handle transaction events
 */
function handleTransactionEvent(data) {
  console.log('Transaction event:');
  console.log(`  ID: ${data.id}`);
  console.log(`  From: ${data.from}`);
  console.log(`  To: ${data.to}`);
  console.log(`  Amount: ${formatTON(data.amount)} TON`);
  console.log(`  Status: ${data.status}`);

  // Your business logic here
  // e.g., update database, send notification, etc.
}

/**
 * Handle balance change events
 */
function handleBalanceChangeEvent(data) {
  console.log('Balance change event:');
  console.log(`  Address: ${data.address}`);
  console.log(`  Old Balance: ${formatTON(data.old_balance)} TON`);
  console.log(`  New Balance: ${formatTON(data.new_balance)} TON`);
  console.log(`  Change: ${formatTON(data.change)} TON`);

  // Your business logic here
}

/**
 * Format nanoTON to TON
 */
function formatTON(nanoTON) {
  return (nanoTON / 1e9).toFixed(4);
}

/**
 * Health check endpoint
 */
app.get('/health', (req, res) => {
  res.json({ status: 'ok' });
});

/**
 * Start server
 */
app.listen(PORT, () => {
  console.log(`Webhook receiver listening on port ${PORT}`);
  console.log(`Webhook endpoint: http://localhost:${PORT}/webhook`);
  console.log('');
  console.log('Register this webhook with tonkey:');
  console.log(`  POST /v1/webhooks`);
  console.log(`  {`);
  console.log(`    "url": "http://your-server:${PORT}/webhook",`);
  console.log(`    "event_type": "transaction",`);
  console.log(`    "secret": "${WEBHOOK_SECRET}"`);
  console.log(`  }`);
});
