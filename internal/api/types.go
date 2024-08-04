package api

type LoginReq struct {
	User string `json:"user"`
	Pass string `json:"pass"`
}

type LoginResp struct {
	Token string `json:"token"`
}

type SendTxReq struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Amount int64  `json:"amount"`
}

type SendTxResp struct {
	ID string `json:"id"`
}

type QueryReq struct {
	Entity  string            `json:"entity"`  // "tx"
	Filters map[string]string `json:"filters"` // e.g. status=submitted
	Limit   int               `json:"limit"`
}

type CreateWebhookReq struct {
	URL           string `json:"url"`
	EventType     string `json:"event_type"`      // "transaction", "balance_change"
	FilterAddress string `json:"filter_address"`  // optional
	Secret        string `json:"secret"`          // for HMAC signature
}

type WebhookResp struct {
	ID            int64  `json:"id"`
	URL           string `json:"url"`
	EventType     string `json:"event_type"`
	FilterAddress string `json:"filter_address,omitempty"`
	IsActive      bool   `json:"is_active"`
	CreatedAt     int64  `json:"created_at"`
}

type CreateUserReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email,omitempty"`
	OrgID    int64  `json:"org_id,omitempty"`
	Role     string `json:"role,omitempty"`  // "user" or "admin"
}

type UpdateUserReq struct {
	Email    string `json:"email,omitempty"`
	Role     string `json:"role,omitempty"`
	IsActive *bool  `json:"is_active,omitempty"`
}

type UserResp struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email,omitempty"`
	OrgID     int64  `json:"org_id,omitempty"`
	Role      string `json:"role"`
	IsActive  bool   `json:"is_active"`
	CreatedAt int64  `json:"created_at"`
	LastLogin int64  `json:"last_login,omitempty"`
}
