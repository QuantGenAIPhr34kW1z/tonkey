PRAGMA journal_mode=WAL;
PRAGMA foreign_keys=ON;

-- Schema migrations tracking 
CREATE TABLE IF NOT EXISTS schema_migrations (
  version INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  applied_at INTEGER NOT NULL
);

-- Organizations for multi-tenancy
CREATE TABLE IF NOT EXISTS organizations (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  slug TEXT UNIQUE NOT NULL,
  plan TEXT NOT NULL DEFAULT 'free',
  is_active BOOLEAN DEFAULT 1,
  created_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_organizations_slug ON organizations(slug);

-- Users with bcrypt password hashing and 2FA support
CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  username TEXT UNIQUE NOT NULL,
  password_hash TEXT NOT NULL,
  email TEXT,
  org_id INTEGER,
  role TEXT NOT NULL DEFAULT 'user',
  is_active BOOLEAN DEFAULT 1,
  created_at INTEGER NOT NULL,
  last_login INTEGER,
  -- 2FA fields
  totp_secret TEXT,
  totp_enabled BOOLEAN DEFAULT 0,
  totp_verified_at INTEGER,
  FOREIGN KEY (org_id) REFERENCES organizations(id)
);

CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_users_org_id ON users(org_id);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

-- 2FA backup codes
CREATE TABLE IF NOT EXISTS backup_codes (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  code_hash TEXT NOT NULL,
  used_at INTEGER,
  created_at INTEGER NOT NULL,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_backup_codes_user_id ON backup_codes(user_id);

-- Password reset tokens 
CREATE TABLE IF NOT EXISTS password_reset_tokens (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  token_hash TEXT UNIQUE NOT NULL,
  expires_at INTEGER NOT NULL,
  used_at INTEGER,
  created_at INTEGER NOT NULL,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_user_id ON password_reset_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_token_hash ON password_reset_tokens(token_hash);

-- Audit logs 
CREATE TABLE IF NOT EXISTS audit_logs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER,
  org_id INTEGER,
  action TEXT NOT NULL,
  resource_type TEXT,
  resource_id TEXT,
  ip_address TEXT,
  user_agent TEXT,
  metadata TEXT,
  checksum TEXT,
  created_at INTEGER NOT NULL,
  FOREIGN KEY (user_id) REFERENCES users(id),
  FOREIGN KEY (org_id) REFERENCES organizations(id)
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_org_id ON audit_logs(org_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource ON audit_logs(resource_type, resource_id);

-- IP rules for allowlist/blocklist 
CREATE TABLE IF NOT EXISTS ip_rules (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  org_id INTEGER,
  cidr TEXT NOT NULL,
  rule_type TEXT NOT NULL CHECK(rule_type IN ('allow', 'block')),
  description TEXT,
  is_active BOOLEAN DEFAULT 1,
  created_at INTEGER NOT NULL,
  expires_at INTEGER,
  FOREIGN KEY (org_id) REFERENCES organizations(id)
);

CREATE INDEX IF NOT EXISTS idx_ip_rules_org_id ON ip_rules(org_id);
CREATE INDEX IF NOT EXISTS idx_ip_rules_rule_type ON ip_rules(rule_type);

-- Transaction log
CREATE TABLE IF NOT EXISTS tx_log (
  id TEXT PRIMARY KEY,
  user_id INTEGER,
  org_id INTEGER,
  created_at INTEGER NOT NULL,
  from_addr TEXT NOT NULL,
  to_addr TEXT NOT NULL,
  amount INTEGER NOT NULL,
  status TEXT NOT NULL,
  batch_id INTEGER,
  FOREIGN KEY (user_id) REFERENCES users(id),
  FOREIGN KEY (org_id) REFERENCES organizations(id),
  FOREIGN KEY (batch_id) REFERENCES tx_batches(id)
);

CREATE INDEX IF NOT EXISTS idx_tx_log_user_id ON tx_log(user_id);
CREATE INDEX IF NOT EXISTS idx_tx_log_org_id ON tx_log(org_id);
CREATE INDEX IF NOT EXISTS idx_tx_log_status ON tx_log(status);
CREATE INDEX IF NOT EXISTS idx_tx_log_batch_id ON tx_log(batch_id);

-- Transaction batches 
CREATE TABLE IF NOT EXISTS tx_batches (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  org_id INTEGER,
  status TEXT NOT NULL DEFAULT 'pending',
  total_amount INTEGER NOT NULL DEFAULT 0,
  tx_count INTEGER NOT NULL DEFAULT 0,
  processed_count INTEGER NOT NULL DEFAULT 0,
  error TEXT,
  created_at INTEGER NOT NULL,
  started_at INTEGER,
  completed_at INTEGER,
  FOREIGN KEY (user_id) REFERENCES users(id),
  FOREIGN KEY (org_id) REFERENCES organizations(id)
);

CREATE INDEX IF NOT EXISTS idx_tx_batches_user_id ON tx_batches(user_id);
CREATE INDEX IF NOT EXISTS idx_tx_batches_status ON tx_batches(status);

-- Smart contracts registry 
CREATE TABLE IF NOT EXISTS contracts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  address TEXT UNIQUE NOT NULL,
  user_id INTEGER NOT NULL,
  org_id INTEGER,
  name TEXT,
  contract_type TEXT,
  abi TEXT,
  state TEXT,
  deployed_at INTEGER,
  created_at INTEGER NOT NULL,
  FOREIGN KEY (user_id) REFERENCES users(id),
  FOREIGN KEY (org_id) REFERENCES organizations(id)
);

CREATE INDEX IF NOT EXISTS idx_contracts_address ON contracts(address);
CREATE INDEX IF NOT EXISTS idx_contracts_user_id ON contracts(user_id);
CREATE INDEX IF NOT EXISTS idx_contracts_org_id ON contracts(org_id);

-- Multi-signature wallets 
CREATE TABLE IF NOT EXISTS multisig_wallets (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  address TEXT UNIQUE NOT NULL,
  org_id INTEGER NOT NULL,
  name TEXT,
  threshold INTEGER NOT NULL,
  signers TEXT NOT NULL,
  is_active BOOLEAN DEFAULT 1,
  created_at INTEGER NOT NULL,
  FOREIGN KEY (org_id) REFERENCES organizations(id)
);

CREATE INDEX IF NOT EXISTS idx_multisig_wallets_address ON multisig_wallets(address);
CREATE INDEX IF NOT EXISTS idx_multisig_wallets_org_id ON multisig_wallets(org_id);

-- Multi-signature proposals 
CREATE TABLE IF NOT EXISTS multisig_proposals (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  wallet_id INTEGER NOT NULL,
  proposer_id INTEGER NOT NULL,
  action_type TEXT NOT NULL,
  action_data TEXT NOT NULL,
  approvals TEXT DEFAULT '[]',
  rejections TEXT DEFAULT '[]',
  status TEXT NOT NULL DEFAULT 'pending',
  tx_hash TEXT,
  created_at INTEGER NOT NULL,
  expires_at INTEGER,
  executed_at INTEGER,
  FOREIGN KEY (wallet_id) REFERENCES multisig_wallets(id),
  FOREIGN KEY (proposer_id) REFERENCES users(id)
);

CREATE INDEX IF NOT EXISTS idx_multisig_proposals_wallet_id ON multisig_proposals(wallet_id);
CREATE INDEX IF NOT EXISTS idx_multisig_proposals_status ON multisig_proposals(status);

-- Watchlist for Telegram bot
CREATE TABLE IF NOT EXISTS watchlist (
  chat_id INTEGER NOT NULL,
  addr TEXT NOT NULL,
  user_id INTEGER,
  created_at INTEGER NOT NULL,
  PRIMARY KEY(chat_id, addr),
  FOREIGN KEY (user_id) REFERENCES users(id)
);

-- API Keys for service-to-service auth 
CREATE TABLE IF NOT EXISTS api_keys (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  org_id INTEGER,
  key_hash TEXT UNIQUE NOT NULL,
  key_prefix TEXT NOT NULL,
  name TEXT NOT NULL,
  scopes TEXT DEFAULT '[]',
  rate_limit INTEGER,
  last_used INTEGER,
  use_count INTEGER DEFAULT 0,
  expires_at INTEGER,
  is_active BOOLEAN DEFAULT 1,
  created_at INTEGER NOT NULL,
  FOREIGN KEY (user_id) REFERENCES users(id),
  FOREIGN KEY (org_id) REFERENCES organizations(id)
);

CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys(user_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_key_hash ON api_keys(key_hash);
CREATE INDEX IF NOT EXISTS idx_api_keys_key_prefix ON api_keys(key_prefix);

-- Webhooks for event notifications
CREATE TABLE IF NOT EXISTS webhooks (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  org_id INTEGER,
  url TEXT NOT NULL,
  event_type TEXT NOT NULL,
  filter_address TEXT,
  secret TEXT NOT NULL,
  is_active BOOLEAN DEFAULT 1,
  failure_count INTEGER DEFAULT 0,
  last_failure TEXT,
  created_at INTEGER NOT NULL,
  FOREIGN KEY (user_id) REFERENCES users(id),
  FOREIGN KEY (org_id) REFERENCES organizations(id)
);

CREATE INDEX IF NOT EXISTS idx_webhooks_user_id ON webhooks(user_id);
CREATE INDEX IF NOT EXISTS idx_webhooks_event_type ON webhooks(event_type);

-- Webhook delivery logs
CREATE TABLE IF NOT EXISTS webhook_deliveries (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  webhook_id INTEGER NOT NULL,
  event_data TEXT NOT NULL,
  status_code INTEGER,
  response_body TEXT,
  error TEXT,
  attempt INTEGER DEFAULT 1,
  next_retry INTEGER,
  delivered_at INTEGER NOT NULL,
  FOREIGN KEY (webhook_id) REFERENCES webhooks(id)
);

CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_webhook_id ON webhook_deliveries(webhook_id);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_next_retry ON webhook_deliveries(next_retry);

-- Background jobs 
CREATE TABLE IF NOT EXISTS jobs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  queue TEXT NOT NULL DEFAULT 'default',
  job_type TEXT NOT NULL,
  payload TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending',
  priority INTEGER DEFAULT 0,
  attempts INTEGER DEFAULT 0,
  max_attempts INTEGER DEFAULT 3,
  last_error TEXT,
  scheduled_at INTEGER NOT NULL,
  started_at INTEGER,
  completed_at INTEGER,
  created_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_jobs_queue_status ON jobs(queue, status);
CREATE INDEX IF NOT EXISTS idx_jobs_scheduled_at ON jobs(scheduled_at);
CREATE INDEX IF NOT EXISTS idx_jobs_job_type ON jobs(job_type);

-- Email verification tokens 
CREATE TABLE IF NOT EXISTS email_verifications (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  email TEXT NOT NULL,
  token_hash TEXT UNIQUE NOT NULL,
  expires_at INTEGER NOT NULL,
  verified_at INTEGER,
  created_at INTEGER NOT NULL,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_email_verifications_user_id ON email_verifications(user_id);
CREATE INDEX IF NOT EXISTS idx_email_verifications_token_hash ON email_verifications(token_hash);
