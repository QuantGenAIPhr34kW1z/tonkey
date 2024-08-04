package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"tonkey/internal/logger"
	"tonkey/internal/metrics"
	"tonkey/internal/store"
)

// Manager handles webhook registration and delivery
type Manager struct {
	DB         *sql.DB
	HTTPClient *http.Client
	deliveries chan *Delivery
	qb         *store.QueryBuilder
}

// Webhook represents a registered webhook
type Webhook struct {
	ID            int64  `json:"id"`
	UserID        int64  `json:"user_id"`
	OrgID         int64  `json:"org_id,omitempty"`
	URL           string `json:"url"`
	EventType     string `json:"event_type"`
	FilterAddress string `json:"filter_address,omitempty"`
	Secret        string `json:"-"`
	IsActive      bool   `json:"is_active"`
	CreatedAt     int64  `json:"created_at"`
}

// Event represents a webhook event
type Event struct {
	Type      string                 `json:"event"`
	Timestamp int64                  `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// Delivery represents a webhook delivery attempt
type Delivery struct {
	WebhookID int64
	Event     *Event
}

// NewManager creates a new webhook manager
func NewManager(db *sql.DB) *Manager {
	return &Manager{
		DB: db,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		deliveries: make(chan *Delivery, 1000),
		qb:         store.NewQueryBuilder(db),
	}
}

// Start begins processing webhook deliveries
func (m *Manager) Start(workers int) {
	for i := 0; i < workers; i++ {
		go m.worker()
	}
	logger.Info.Printf("Started %d webhook workers", workers)
}

// worker processes webhook deliveries
func (m *Manager) worker() {
	for delivery := range m.deliveries {
		m.deliver(delivery)
	}
}

// CreateWebhook registers a new webhook
func (m *Manager) CreateWebhook(userID, orgID int64, url, eventType, filterAddress, secret string) (*Webhook, error) {
	if secret == "" {
		return nil, fmt.Errorf("secret is required")
	}

	query := m.qb.Build(`
		INSERT INTO webhooks (user_id, org_id, url, event_type, filter_address, secret, is_active, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, 8)
	result, err := m.DB.Exec(query, userID, orgID, url, eventType, filterAddress, secret, true, time.Now().Unix())
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &Webhook{
		ID:            id,
		UserID:        userID,
		OrgID:         orgID,
		URL:           url,
		EventType:     eventType,
		FilterAddress: filterAddress,
		IsActive:      true,
		CreatedAt:     time.Now().Unix(),
	}, nil
}

// GetWebhook retrieves a webhook by ID
func (m *Manager) GetWebhook(id int64) (*Webhook, error) {
	var wh Webhook
	query := m.qb.Build(`
		SELECT id, user_id, org_id, url, event_type, filter_address, secret, is_active, created_at
		FROM webhooks
		WHERE id = ?
	`, 1)
	err := m.DB.QueryRow(query, id).Scan(&wh.ID, &wh.UserID, &wh.OrgID, &wh.URL, &wh.EventType,
		&wh.FilterAddress, &wh.Secret, &wh.IsActive, &wh.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &wh, nil
}

// ListWebhooks retrieves all webhooks for a user
func (m *Manager) ListWebhooks(userID int64) ([]*Webhook, error) {
	query := m.qb.Build(`
		SELECT id, user_id, org_id, url, event_type, filter_address, is_active, created_at
		FROM webhooks
		WHERE user_id = ?
		ORDER BY created_at DESC
	`, 1)
	rows, err := m.DB.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var webhooks []*Webhook
	for rows.Next() {
		var wh Webhook
		err := rows.Scan(&wh.ID, &wh.UserID, &wh.OrgID, &wh.URL, &wh.EventType,
			&wh.FilterAddress, &wh.IsActive, &wh.CreatedAt)
		if err != nil {
			return nil, err
		}
		webhooks = append(webhooks, &wh)
	}
	return webhooks, rows.Err()
}

// DeleteWebhook deletes a webhook
func (m *Manager) DeleteWebhook(id, userID int64) error {
	query := m.qb.Build("DELETE FROM webhooks WHERE id = ? AND user_id = ?", 2)
	result, err := m.DB.Exec(query, id, userID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("webhook not found")
	}
	return nil
}

// Trigger queues webhook deliveries for an event
func (m *Manager) Trigger(eventType string, data map[string]interface{}) {
	address := ""
	if addr, ok := data["address"].(string); ok {
		address = addr
	} else if from, ok := data["from"].(string); ok {
		address = from
	}

	// Find matching webhooks
	queryStr := `
		SELECT id, url, secret
		FROM webhooks
		WHERE is_active = 1 AND event_type = ?
	`
	args := []interface{}{eventType}

	if address != "" {
		queryStr += " AND (filter_address = ? OR filter_address = '')"
		args = append(args, address)
	}

	queryStr = m.qb.Build(queryStr, len(args))
	rows, err := m.DB.Query(queryStr, args...)
	if err != nil {
		logger.Error.Printf("Failed to query webhooks: %v", err)
		return
	}
	defer rows.Close()

	event := &Event{
		Type:      eventType,
		Timestamp: time.Now().Unix(),
		Data:      data,
	}

	for rows.Next() {
		var id int64
		var url, secret string
		if err := rows.Scan(&id, &url, &secret); err != nil {
			logger.Error.Printf("Failed to scan webhook: %v", err)
			continue
		}

		// Queue delivery
		select {
		case m.deliveries <- &Delivery{WebhookID: id, Event: event}:
		default:
			logger.Warn.Printf("Webhook delivery queue full, dropping event for webhook %d", id)
		}
	}
}

// deliver attempts to deliver a webhook event
func (m *Manager) deliver(delivery *Delivery) {
	start := time.Now()

	// Get webhook details
	wh, err := m.GetWebhook(delivery.WebhookID)
	if err != nil {
		logger.Error.Printf("Failed to get webhook %d: %v", delivery.WebhookID, err)
		return
	}

	// Serialize event
	body, err := json.Marshal(delivery.Event)
	if err != nil {
		logger.Error.Printf("Failed to marshal event: %v", err)
		return
	}

	// Create signature
	signature := m.sign(body, wh.Secret)

	// Send HTTP request
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", wh.URL, bytes.NewReader(body))
	if err != nil {
		m.logDelivery(wh.ID, body, 0, "", err.Error())
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tonkey-Signature", "sha256="+signature)
	req.Header.Set("X-Tonkey-Event", delivery.Event.Type)

	resp, err := m.HTTPClient.Do(req)
	if err != nil {
		m.logDelivery(wh.ID, body, 0, "", err.Error())
		metrics.RecordWebhookDelivery(false, time.Since(start))
		return
	}
	defer resp.Body.Close()

	// Read response
	respBody := make([]byte, 1024)
	n, _ := resp.Body.Read(respBody)
	respBody = respBody[:n]

	success := resp.StatusCode >= 200 && resp.StatusCode < 300
	m.logDelivery(wh.ID, body, resp.StatusCode, string(respBody), "")
	metrics.RecordWebhookDelivery(success, time.Since(start))

	if success {
		logger.Debug.Printf("Webhook %d delivered successfully", wh.ID)
	} else {
		logger.Warn.Printf("Webhook %d delivery failed with status %d", wh.ID, resp.StatusCode)
	}
}

// sign creates HMAC signature
func (m *Manager) sign(body []byte, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}

// logDelivery records a delivery attempt
func (m *Manager) logDelivery(webhookID int64, eventData []byte, statusCode int, response, errMsg string) {
	query := m.qb.Build(`
		INSERT INTO webhook_deliveries (webhook_id, event_data, status_code, response_body, error, delivered_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, 6)
	_, err := m.DB.Exec(query, webhookID, string(eventData), statusCode, response, errMsg, time.Now().Unix())
	if err != nil {
		logger.Error.Printf("Failed to log webhook delivery: %v", err)
	}
}

// VerifySignature verifies a webhook signature
func VerifySignature(body []byte, signature, secret string) bool {
	expected := hmac.New(sha256.New, []byte(secret))
	expected.Write(body)
	expectedSig := "sha256=" + hex.EncodeToString(expected.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(expectedSig))
}
