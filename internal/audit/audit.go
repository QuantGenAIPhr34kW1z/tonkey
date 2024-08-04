// Package audit provides audit logging functionality for security and compliance.
package audit

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Action types for audit logging.
const (
	ActionLogin            = "login"
	ActionLoginFailed      = "login_failed"
	ActionLogout           = "logout"
	ActionPasswordChange   = "password_change"
	ActionPasswordReset    = "password_reset"
	ActionTOTPSetup        = "totp_setup"
	ActionTOTPVerify       = "totp_verify"
	ActionTOTPDisable      = "totp_disable"
	ActionUserCreate       = "user_create"
	ActionUserUpdate       = "user_update"
	ActionUserDelete       = "user_delete"
	ActionOrgCreate        = "org_create"
	ActionOrgUpdate        = "org_update"
	ActionOrgDelete        = "org_delete"
	ActionTxSend           = "tx_send"
	ActionTxBatch          = "tx_batch"
	ActionWebhookCreate    = "webhook_create"
	ActionWebhookDelete    = "webhook_delete"
	ActionAPIKeyCreate     = "api_key_create"
	ActionAPIKeyRevoke     = "api_key_revoke"
	ActionContractDeploy   = "contract_deploy"
	ActionContractCall     = "contract_call"
	ActionMultisigCreate   = "multisig_create"
	ActionMultisigPropose  = "multisig_propose"
	ActionMultisigApprove  = "multisig_approve"
	ActionMultisigReject   = "multisig_reject"
	ActionMultisigExecute  = "multisig_execute"
	ActionIPRuleCreate     = "ip_rule_create"
	ActionIPRuleDelete     = "ip_rule_delete"
	ActionRegistration     = "registration"
	ActionEmailVerify      = "email_verify"
)

// ResourceType defines the type of resource being audited.
const (
	ResourceUser       = "user"
	ResourceOrg        = "organization"
	ResourceTx         = "transaction"
	ResourceWebhook    = "webhook"
	ResourceAPIKey     = "api_key"
	ResourceContract   = "contract"
	ResourceMultisig   = "multisig"
	ResourceIPRule     = "ip_rule"
)

// Entry represents an audit log entry.
type Entry struct {
	ID           int64             `json:"id"`
	UserID       *int64            `json:"user_id,omitempty"`
	OrgID        *int64            `json:"org_id,omitempty"`
	Action       string            `json:"action"`
	ResourceType string            `json:"resource_type,omitempty"`
	ResourceID   string            `json:"resource_id,omitempty"`
	IPAddress    string            `json:"ip_address,omitempty"`
	UserAgent    string            `json:"user_agent,omitempty"`
	Metadata     map[string]any    `json:"metadata,omitempty"`
	Checksum     string            `json:"checksum"`
	CreatedAt    int64             `json:"created_at"`
}

// Logger handles audit logging operations.
type Logger struct {
	db      *sql.DB
	enabled bool
}

// NewLogger creates a new audit logger.
func NewLogger(db *sql.DB, enabled bool) *Logger {
	return &Logger{
		db:      db,
		enabled: enabled,
	}
}

// Log records an audit entry.
func (l *Logger) Log(entry Entry) error {
	if !l.enabled {
		return nil
	}

	entry.CreatedAt = time.Now().Unix()
	entry.Checksum = l.computeChecksum(entry)

	var metadataJSON []byte
	var err error
	if entry.Metadata != nil {
		metadataJSON, err = json.Marshal(entry.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	_, err = l.db.Exec(`
		INSERT INTO audit_logs (user_id, org_id, action, resource_type, resource_id,
			ip_address, user_agent, metadata, checksum, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.UserID, entry.OrgID, entry.Action, entry.ResourceType, entry.ResourceID,
		entry.IPAddress, entry.UserAgent, string(metadataJSON), entry.Checksum, entry.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert audit log: %w", err)
	}

	return nil
}

// LogAction is a convenience method for common audit logging.
func (l *Logger) LogAction(userID *int64, orgID *int64, action, resourceType, resourceID, ip, userAgent string, metadata map[string]any) error {
	return l.Log(Entry{
		UserID:       userID,
		OrgID:        orgID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		IPAddress:    ip,
		UserAgent:    userAgent,
		Metadata:     metadata,
	})
}

// Query represents audit log query parameters.
type Query struct {
	UserID       *int64
	OrgID        *int64
	Action       string
	ResourceType string
	ResourceID   string
	StartTime    *int64
	EndTime      *int64
	Limit        int
	Offset       int
}

// Search searches audit logs with filters.
func (l *Logger) Search(q Query) ([]Entry, int, error) {
	if q.Limit <= 0 || q.Limit > 1000 {
		q.Limit = 100
	}

	// Build query
	baseQuery := "FROM audit_logs WHERE 1=1"
	args := []any{}

	if q.UserID != nil {
		baseQuery += " AND user_id = ?"
		args = append(args, *q.UserID)
	}
	if q.OrgID != nil {
		baseQuery += " AND org_id = ?"
		args = append(args, *q.OrgID)
	}
	if q.Action != "" {
		baseQuery += " AND action = ?"
		args = append(args, q.Action)
	}
	if q.ResourceType != "" {
		baseQuery += " AND resource_type = ?"
		args = append(args, q.ResourceType)
	}
	if q.ResourceID != "" {
		baseQuery += " AND resource_id = ?"
		args = append(args, q.ResourceID)
	}
	if q.StartTime != nil {
		baseQuery += " AND created_at >= ?"
		args = append(args, *q.StartTime)
	}
	if q.EndTime != nil {
		baseQuery += " AND created_at <= ?"
		args = append(args, *q.EndTime)
	}

	// Get total count
	var total int
	countQuery := "SELECT COUNT(*) " + baseQuery
	if err := l.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count audit logs: %w", err)
	}

	// Get entries
	selectQuery := "SELECT id, user_id, org_id, action, resource_type, resource_id, ip_address, user_agent, metadata, checksum, created_at " + baseQuery + " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, q.Limit, q.Offset)

	rows, err := l.db.Query(selectQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query audit logs: %w", err)
	}
	defer rows.Close()

	entries := []Entry{}
	for rows.Next() {
		var e Entry
		var userID, orgID sql.NullInt64
		var resourceType, resourceID, ipAddress, userAgent, metadata sql.NullString

		err := rows.Scan(
			&e.ID, &userID, &orgID, &e.Action, &resourceType, &resourceID,
			&ipAddress, &userAgent, &metadata, &e.Checksum, &e.CreatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan audit log: %w", err)
		}

		if userID.Valid {
			e.UserID = &userID.Int64
		}
		if orgID.Valid {
			e.OrgID = &orgID.Int64
		}
		if resourceType.Valid {
			e.ResourceType = resourceType.String
		}
		if resourceID.Valid {
			e.ResourceID = resourceID.String
		}
		if ipAddress.Valid {
			e.IPAddress = ipAddress.String
		}
		if userAgent.Valid {
			e.UserAgent = userAgent.String
		}
		if metadata.Valid && metadata.String != "" {
			json.Unmarshal([]byte(metadata.String), &e.Metadata)
		}

		entries = append(entries, e)
	}

	return entries, total, nil
}

// GetByID retrieves a single audit entry.
func (l *Logger) GetByID(id int64) (*Entry, error) {
	var e Entry
	var userID, orgID sql.NullInt64
	var resourceType, resourceID, ipAddress, userAgent, metadata sql.NullString

	err := l.db.QueryRow(`
		SELECT id, user_id, org_id, action, resource_type, resource_id,
			ip_address, user_agent, metadata, checksum, created_at
		FROM audit_logs WHERE id = ?`, id,
	).Scan(
		&e.ID, &userID, &orgID, &e.Action, &resourceType, &resourceID,
		&ipAddress, &userAgent, &metadata, &e.Checksum, &e.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if userID.Valid {
		e.UserID = &userID.Int64
	}
	if orgID.Valid {
		e.OrgID = &orgID.Int64
	}
	if resourceType.Valid {
		e.ResourceType = resourceType.String
	}
	if resourceID.Valid {
		e.ResourceID = resourceID.String
	}
	if ipAddress.Valid {
		e.IPAddress = ipAddress.String
	}
	if userAgent.Valid {
		e.UserAgent = userAgent.String
	}
	if metadata.Valid && metadata.String != "" {
		json.Unmarshal([]byte(metadata.String), &e.Metadata)
	}

	return &e, nil
}

// VerifyChecksum verifies the integrity of an audit entry.
func (l *Logger) VerifyChecksum(entry Entry) bool {
	expected := l.computeChecksum(entry)
	return entry.Checksum == expected
}

// computeChecksum creates a checksum for an audit entry.
func (l *Logger) computeChecksum(entry Entry) string {
	data := fmt.Sprintf("%v:%v:%s:%s:%s:%d",
		entry.UserID, entry.OrgID, entry.Action, entry.ResourceType, entry.ResourceID, entry.CreatedAt)
	h := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", h[:16])
}

// Cleanup removes old audit logs based on retention policy.
func (l *Logger) Cleanup(retentionDays int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays).Unix()
	result, err := l.db.Exec("DELETE FROM audit_logs WHERE created_at < ?", cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup audit logs: %w", err)
	}
	return result.RowsAffected()
}

// ExportJSON exports audit logs as JSON.
func (l *Logger) ExportJSON(q Query) ([]byte, error) {
	entries, _, err := l.Search(q)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(entries, "", "  ")
}
