/**
 * tonkey API Client for JavaScript/React applications
 */

export class TonkeyClient {
  constructor(baseURL = 'http://localhost:8080') {
    this.baseURL = baseURL;
    this.token = null;
  }

  /**
   * Set the JWT token for authenticated requests
   */
  setToken(token) {
    this.token = token;
    localStorage.setItem('tonkey_token', token);
  }

  /**
   * Get stored token
   */
  getToken() {
    if (!this.token) {
      this.token = localStorage.getItem('tonkey_token');
    }
    return this.token;
  }

  /**
   * Clear token (logout)
   */
  clearToken() {
    this.token = null;
    localStorage.removeItem('tonkey_token');
  }

  /**
   * Make authenticated request
   */
  async request(method, path, body = null) {
    const headers = {
      'Content-Type': 'application/json',
    };

    if (this.getToken()) {
      headers['Authorization'] = `Bearer ${this.getToken()}`;
    }

    const options = {
      method,
      headers,
    };

    if (body) {
      options.body = JSON.stringify(body);
    }

    const response = await fetch(`${this.baseURL}${path}`, options);

    if (!response.ok) {
      const error = await response.text();
      throw new Error(error || `Request failed: ${response.status}`);
    }

    return response.json();
  }

  /**
   * Login with username and password
   */
  async login(username, password) {
    const data = await this.request('POST', '/auth/login', {
      user: username,
      pass: password,
    });
    this.setToken(data.token);
    return data.token;
  }

  /**
   * Get wallet balance
   */
  async getBalance(address) {
    return this.request('GET', `/v1/wallet/${address}/balance`);
  }

  /**
   * Send transaction
   */
  async sendTransaction(from, to, amount) {
    return this.request('POST', '/v1/tx/send', { from, to, amount });
  }

  /**
   * Get transaction by ID
   */
  async getTransaction(id) {
    return this.request('GET', `/v1/tx/${id}`);
  }

  /**
   * Query transactions
   */
  async queryTransactions(filters = {}, limit = 50) {
    return this.request('POST', '/v1/query', {
      entity: 'tx',
      filters,
      limit,
    });
  }

  /**
   * List webhooks
   */
  async listWebhooks() {
    return this.request('GET', '/v1/webhooks');
  }

  /**
   * Create webhook
   */
  async createWebhook(url, eventType, filterAddress, secret) {
    return this.request('POST', '/v1/webhooks', {
      url,
      event_type: eventType,
      filter_address: filterAddress,
      secret,
    });
  }

  /**
   * Delete webhook
   */
  async deleteWebhook(id) {
    return this.request('DELETE', `/v1/webhooks/${id}`);
  }

  /**
   * Setup 2FA
   */
  async setup2FA() {
    return this.request('POST', '/auth/2fa/setup');
  }

  /**
   * Verify 2FA
   */
  async verify2FA(code) {
    return this.request('POST', '/auth/2fa/verify', { code });
  }

  /**
   * List API keys
   */
  async listAPIKeys() {
    return this.request('GET', '/v1/api-keys');
  }

  /**
   * Create API key
   */
  async createAPIKey(name, scopes = [], expiresIn = null) {
    return this.request('POST', '/v1/api-keys', {
      name,
      scopes,
      expires_in: expiresIn,
    });
  }

  /**
   * Create WebSocket connection
   */
  createWebSocket() {
    const token = this.getToken();
    if (!token) {
      throw new Error('Not authenticated');
    }

    const wsURL = this.baseURL.replace('http', 'ws') + `/ws?token=${token}`;
    return new WebSocket(wsURL);
  }
}

// React hook for using the client
export function useTonkey() {
  const client = new TonkeyClient(process.env.REACT_APP_API_URL || 'http://localhost:8080');
  return client;
}

// Format nanoTON to TON
export function formatTON(nanoTON) {
  return (nanoTON / 1e9).toFixed(2);
}

// Format TON address (truncate)
export function formatAddress(address, length = 8) {
  if (!address || address.length <= length * 2) return address;
  return `${address.slice(0, length)}...${address.slice(-length)}`;
}

export default TonkeyClient;
