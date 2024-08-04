// Package openapi provides OpenAPI/Swagger documentation for the tonkey API.
package openapi

import (
	_ "embed"
	"encoding/json"
	"html/template"
	"net/http"
)

//go:embed spec.json
var specJSON []byte

//go:embed swagger-ui.html
var swaggerUIHTML string

// Spec represents the OpenAPI specification.
type Spec struct {
	OpenAPI    string              `json:"openapi"`
	Info       Info                `json:"info"`
	Servers    []Server            `json:"servers,omitempty"`
	Paths      map[string]PathItem `json:"paths"`
	Components Components          `json:"components,omitempty"`
	Tags       []Tag               `json:"tags,omitempty"`
}

// Info provides metadata about the API.
type Info struct {
	Title          string   `json:"title"`
	Description    string   `json:"description,omitempty"`
	Version        string   `json:"version"`
	TermsOfService string   `json:"termsOfService,omitempty"`
	Contact        *Contact `json:"contact,omitempty"`
}

// Contact information for the API.
type Contact struct {
	Name  string `json:"name,omitempty"`
	URL   string `json:"url,omitempty"`
	Email string `json:"email,omitempty"`
}

// Server represents a server URL.
type Server struct {
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
}

// PathItem describes the operations available on a single path.
type PathItem struct {
	Get    *Operation `json:"get,omitempty"`
	Post   *Operation `json:"post,omitempty"`
	Put    *Operation `json:"put,omitempty"`
	Delete *Operation `json:"delete,omitempty"`
	Patch  *Operation `json:"patch,omitempty"`
}

// Operation describes a single API operation on a path.
type Operation struct {
	Tags        []string              `json:"tags,omitempty"`
	Summary     string                `json:"summary,omitempty"`
	Description string                `json:"description,omitempty"`
	OperationID string                `json:"operationId,omitempty"`
	Parameters  []Parameter           `json:"parameters,omitempty"`
	RequestBody *RequestBody          `json:"requestBody,omitempty"`
	Responses   map[string]Response   `json:"responses"`
	Security    []SecurityRequirement `json:"security,omitempty"`
}

// Parameter describes a single operation parameter.
type Parameter struct {
	Name        string  `json:"name"`
	In          string  `json:"in"` // query, header, path, cookie
	Description string  `json:"description,omitempty"`
	Required    bool    `json:"required,omitempty"`
	Schema      *Schema `json:"schema,omitempty"`
}

// RequestBody describes a request body.
type RequestBody struct {
	Description string               `json:"description,omitempty"`
	Required    bool                 `json:"required,omitempty"`
	Content     map[string]MediaType `json:"content"`
}

// Response describes a single response from an API operation.
type Response struct {
	Description string               `json:"description"`
	Content     map[string]MediaType `json:"content,omitempty"`
}

// MediaType provides schema for a media type.
type MediaType struct {
	Schema  *Schema `json:"schema,omitempty"`
	Example any     `json:"example,omitempty"`
}

// Schema represents a data model.
type Schema struct {
	Type        string             `json:"type,omitempty"`
	Format      string             `json:"format,omitempty"`
	Description string             `json:"description,omitempty"`
	Properties  map[string]*Schema `json:"properties,omitempty"`
	Items       *Schema            `json:"items,omitempty"`
	Required    []string           `json:"required,omitempty"`
	Ref         string             `json:"$ref,omitempty"`
	Enum        []string           `json:"enum,omitempty"`
	Default     any                `json:"default,omitempty"`
	Minimum     *float64           `json:"minimum,omitempty"`
	Maximum     *float64           `json:"maximum,omitempty"`
}

// Components holds reusable objects.
type Components struct {
	Schemas         map[string]*Schema         `json:"schemas,omitempty"`
	SecuritySchemes map[string]*SecurityScheme `json:"securitySchemes,omitempty"`
}

// SecurityScheme defines a security scheme.
type SecurityScheme struct {
	Type         string `json:"type"`
	Scheme       string `json:"scheme,omitempty"`
	BearerFormat string `json:"bearerFormat,omitempty"`
	Name         string `json:"name,omitempty"`
	In           string `json:"in,omitempty"`
	Description  string `json:"description,omitempty"`
}

// SecurityRequirement specifies the security schemes to use.
type SecurityRequirement map[string][]string

// Tag adds metadata to a single tag.
type Tag struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// Handler returns an HTTP handler that serves the OpenAPI spec.
func Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write(specJSON)
	}
}

// UIHandler returns an HTTP handler that serves the Swagger UI.
func UIHandler(specURL string) http.HandlerFunc {
	tmpl := template.Must(template.New("swagger").Parse(swaggerUIHTML))

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		tmpl.Execute(w, map[string]string{
			"SpecURL": specURL,
		})
	}
}

// GenerateSpec generates the OpenAPI specification for tonkey.
func GenerateSpec(baseURL string) *Spec {
	return &Spec{
		OpenAPI: "3.0.3",
		Info: Info{
			Title:       "tonkey API",
			Description: "Production-ready TON blockchain gateway API. Provides wallet management, transaction handling, webhooks, and real-time updates.",
			Version:     "3.4.0",
			Contact: &Contact{
				Name:  "tonkey Team",
				URL:   "https://github.com/tonkey",
				Email: "support@tonkey.io",
			},
		},
		Servers: []Server{
			{URL: baseURL, Description: "Current server"},
			{URL: "http://localhost:8080", Description: "Local development"},
		},
		Tags: []Tag{
			{Name: "auth", Description: "Authentication endpoints"},
			{Name: "wallet", Description: "Wallet operations"},
			{Name: "transaction", Description: "Transaction management"},
			{Name: "webhook", Description: "Webhook subscriptions"},
			{Name: "2fa", Description: "Two-factor authentication"},
			{Name: "contract", Description: "Smart contract interactions"},
			{Name: "multisig", Description: "Multi-signature wallets"},
			{Name: "batch", Description: "Transaction batching"},
			{Name: "apikey", Description: "API key management"},
			{Name: "admin", Description: "Administration endpoints"},
		},
		Paths: generatePaths(),
		Components: Components{
			Schemas:         generateSchemas(),
			SecuritySchemes: generateSecuritySchemes(),
		},
	}
}

func generateSecuritySchemes() map[string]*SecurityScheme {
	return map[string]*SecurityScheme{
		"bearerAuth": {
			Type:         "http",
			Scheme:       "bearer",
			BearerFormat: "JWT",
			Description:  "JWT token authentication",
		},
		"apiKeyAuth": {
			Type:        "apiKey",
			In:          "header",
			Name:        "X-API-Key",
			Description: "API key authentication",
		},
	}
}

func generateSchemas() map[string]*Schema {
	return map[string]*Schema{
		"LoginRequest": {
			Type: "object",
			Properties: map[string]*Schema{
				"user": {Type: "string", Description: "Username"},
				"pass": {Type: "string", Description: "Password"},
			},
			Required: []string{"user", "pass"},
		},
		"LoginResponse": {
			Type: "object",
			Properties: map[string]*Schema{
				"token": {Type: "string", Description: "JWT token"},
			},
		},
		"BalanceResponse": {
			Type: "object",
			Properties: map[string]*Schema{
				"address": {Type: "string", Description: "TON address"},
				"balance": {Type: "integer", Format: "int64", Description: "Balance in nanoTON"},
			},
		},
		"SendTxRequest": {
			Type: "object",
			Properties: map[string]*Schema{
				"from":   {Type: "string", Description: "Sender address"},
				"to":     {Type: "string", Description: "Recipient address"},
				"amount": {Type: "integer", Format: "int64", Description: "Amount in nanoTON"},
			},
			Required: []string{"from", "to", "amount"},
		},
		"SendTxResponse": {
			Type: "object",
			Properties: map[string]*Schema{
				"id": {Type: "string", Description: "Transaction ID"},
			},
		},
		"Transaction": {
			Type: "object",
			Properties: map[string]*Schema{
				"id":         {Type: "string"},
				"created_at": {Type: "integer", Format: "int64"},
				"from":       {Type: "string"},
				"to":         {Type: "string"},
				"amount":     {Type: "integer", Format: "int64"},
				"status":     {Type: "string", Enum: []string{"pending", "submitted", "confirmed", "failed"}},
			},
		},
		"WebhookRequest": {
			Type: "object",
			Properties: map[string]*Schema{
				"url":            {Type: "string", Format: "uri", Description: "Webhook callback URL"},
				"event_type":     {Type: "string", Enum: []string{"transaction", "balance_change"}},
				"filter_address": {Type: "string", Description: "Optional address filter"},
				"secret":         {Type: "string", Description: "Secret for HMAC signing"},
			},
			Required: []string{"url", "event_type", "secret"},
		},
		"Webhook": {
			Type: "object",
			Properties: map[string]*Schema{
				"id":             {Type: "integer", Format: "int64"},
				"url":            {Type: "string"},
				"event_type":     {Type: "string"},
				"filter_address": {Type: "string"},
				"is_active":      {Type: "boolean"},
				"created_at":     {Type: "integer", Format: "int64"},
			},
		},
		"Error": {
			Type: "object",
			Properties: map[string]*Schema{
				"error":   {Type: "string", Description: "Error message"},
				"code":    {Type: "string", Description: "Error code"},
				"details": {Type: "object", Description: "Additional error details"},
			},
		},
		"APIKey": {
			Type: "object",
			Properties: map[string]*Schema{
				"id":         {Type: "integer", Format: "int64"},
				"name":       {Type: "string"},
				"key_prefix": {Type: "string"},
				"scopes":     {Type: "array", Items: &Schema{Type: "string"}},
				"is_active":  {Type: "boolean"},
				"created_at": {Type: "integer", Format: "int64"},
				"expires_at": {Type: "integer", Format: "int64"},
				"use_count":  {Type: "integer"},
			},
		},
		"Contract": {
			Type: "object",
			Properties: map[string]*Schema{
				"id":            {Type: "integer", Format: "int64"},
				"address":       {Type: "string"},
				"name":          {Type: "string"},
				"contract_type": {Type: "string"},
				"state":         {Type: "string"},
				"created_at":    {Type: "integer", Format: "int64"},
			},
		},
		"MultisigWallet": {
			Type: "object",
			Properties: map[string]*Schema{
				"id":        {Type: "integer", Format: "int64"},
				"address":   {Type: "string"},
				"name":      {Type: "string"},
				"threshold": {Type: "integer"},
				"signers":   {Type: "array", Items: &Schema{Type: "string"}},
				"is_active": {Type: "boolean"},
			},
		},
		"Proposal": {
			Type: "object",
			Properties: map[string]*Schema{
				"id":          {Type: "integer", Format: "int64"},
				"wallet_id":   {Type: "integer", Format: "int64"},
				"action_type": {Type: "string"},
				"action_data": {Type: "object"},
				"approvals":   {Type: "array", Items: &Schema{Type: "integer"}},
				"rejections":  {Type: "array", Items: &Schema{Type: "integer"}},
				"status":      {Type: "string", Enum: []string{"pending", "approved", "rejected", "executed", "expired"}},
				"created_at":  {Type: "integer", Format: "int64"},
				"expires_at":  {Type: "integer", Format: "int64"},
			},
		},
		"Batch": {
			Type: "object",
			Properties: map[string]*Schema{
				"id":              {Type: "integer", Format: "int64"},
				"status":          {Type: "string", Enum: []string{"pending", "processing", "completed", "failed", "cancelled"}},
				"total_amount":    {Type: "integer", Format: "int64"},
				"tx_count":        {Type: "integer"},
				"processed_count": {Type: "integer"},
				"created_at":      {Type: "integer", Format: "int64"},
			},
		},
	}
}

func generatePaths() map[string]PathItem {
	jwtSecurity := []SecurityRequirement{{"bearerAuth": {}}}

	return map[string]PathItem{
		"/healthz": {
			Get: &Operation{
				Tags:        []string{"system"},
				Summary:     "Health check",
				Description: "Returns OK if the service is healthy",
				OperationID: "healthCheck",
				Responses: map[string]Response{
					"200": {Description: "Service is healthy"},
				},
			},
		},
		"/auth/login": {
			Post: &Operation{
				Tags:        []string{"auth"},
				Summary:     "Login",
				Description: "Authenticate with username and password to receive a JWT token",
				OperationID: "login",
				RequestBody: &RequestBody{
					Required: true,
					Content: map[string]MediaType{
						"application/json": {Schema: &Schema{Ref: "#/components/schemas/LoginRequest"}},
					},
				},
				Responses: map[string]Response{
					"200": {
						Description: "Successful login",
						Content: map[string]MediaType{
							"application/json": {Schema: &Schema{Ref: "#/components/schemas/LoginResponse"}},
						},
					},
					"401": {Description: "Invalid credentials"},
				},
			},
		},
		"/v1/wallet/{address}/balance": {
			Get: &Operation{
				Tags:        []string{"wallet"},
				Summary:     "Get wallet balance",
				Description: "Query the balance of a TON wallet address",
				OperationID: "getBalance",
				Security:    jwtSecurity,
				Parameters: []Parameter{
					{Name: "address", In: "path", Required: true, Schema: &Schema{Type: "string"}, Description: "TON wallet address"},
				},
				Responses: map[string]Response{
					"200": {
						Description: "Balance retrieved successfully",
						Content: map[string]MediaType{
							"application/json": {Schema: &Schema{Ref: "#/components/schemas/BalanceResponse"}},
						},
					},
					"400": {Description: "Invalid address format"},
					"401": {Description: "Unauthorized"},
					"502": {Description: "Provider error"},
				},
			},
		},
		"/v1/tx/send": {
			Post: &Operation{
				Tags:        []string{"transaction"},
				Summary:     "Send transaction",
				Description: "Submit a transaction to the TON blockchain",
				OperationID: "sendTransaction",
				Security:    jwtSecurity,
				RequestBody: &RequestBody{
					Required: true,
					Content: map[string]MediaType{
						"application/json": {Schema: &Schema{Ref: "#/components/schemas/SendTxRequest"}},
					},
				},
				Responses: map[string]Response{
					"200": {
						Description: "Transaction submitted",
						Content: map[string]MediaType{
							"application/json": {Schema: &Schema{Ref: "#/components/schemas/SendTxResponse"}},
						},
					},
					"400": {Description: "Invalid request"},
					"401": {Description: "Unauthorized"},
					"502": {Description: "Provider error"},
				},
			},
		},
		"/v1/tx/{id}": {
			Get: &Operation{
				Tags:        []string{"transaction"},
				Summary:     "Get transaction",
				Description: "Retrieve transaction details by ID",
				OperationID: "getTransaction",
				Security:    jwtSecurity,
				Parameters: []Parameter{
					{Name: "id", In: "path", Required: true, Schema: &Schema{Type: "string"}, Description: "Transaction ID"},
				},
				Responses: map[string]Response{
					"200": {
						Description: "Transaction found",
						Content: map[string]MediaType{
							"application/json": {Schema: &Schema{Ref: "#/components/schemas/Transaction"}},
						},
					},
					"404": {Description: "Transaction not found"},
				},
			},
		},
		"/v1/webhooks": {
			Get: &Operation{
				Tags:        []string{"webhook"},
				Summary:     "List webhooks",
				Description: "List all webhook subscriptions for the authenticated user",
				OperationID: "listWebhooks",
				Security:    jwtSecurity,
				Responses: map[string]Response{
					"200": {
						Description: "Webhooks listed",
						Content: map[string]MediaType{
							"application/json": {
								Schema: &Schema{
									Type: "object",
									Properties: map[string]*Schema{
										"webhooks": {Type: "array", Items: &Schema{Ref: "#/components/schemas/Webhook"}},
										"count":    {Type: "integer"},
									},
								},
							},
						},
					},
				},
			},
			Post: &Operation{
				Tags:        []string{"webhook"},
				Summary:     "Create webhook",
				Description: "Create a new webhook subscription",
				OperationID: "createWebhook",
				Security:    jwtSecurity,
				RequestBody: &RequestBody{
					Required: true,
					Content: map[string]MediaType{
						"application/json": {Schema: &Schema{Ref: "#/components/schemas/WebhookRequest"}},
					},
				},
				Responses: map[string]Response{
					"201": {
						Description: "Webhook created",
						Content: map[string]MediaType{
							"application/json": {Schema: &Schema{Ref: "#/components/schemas/Webhook"}},
						},
					},
					"400": {Description: "Invalid request"},
				},
			},
		},
		"/v1/webhooks/{id}": {
			Delete: &Operation{
				Tags:        []string{"webhook"},
				Summary:     "Delete webhook",
				Description: "Delete a webhook subscription",
				OperationID: "deleteWebhook",
				Security:    jwtSecurity,
				Parameters: []Parameter{
					{Name: "id", In: "path", Required: true, Schema: &Schema{Type: "integer"}, Description: "Webhook ID"},
				},
				Responses: map[string]Response{
					"200": {Description: "Webhook deleted"},
					"404": {Description: "Webhook not found"},
				},
			},
		},
		"/v1/api-keys": {
			Get: &Operation{
				Tags:        []string{"apikey"},
				Summary:     "List API keys",
				Description: "List all API keys for the authenticated user",
				OperationID: "listAPIKeys",
				Security:    jwtSecurity,
				Responses: map[string]Response{
					"200": {
						Description: "API keys listed",
						Content: map[string]MediaType{
							"application/json": {Schema: &Schema{Type: "array", Items: &Schema{Ref: "#/components/schemas/APIKey"}}},
						},
					},
				},
			},
			Post: &Operation{
				Tags:        []string{"apikey"},
				Summary:     "Create API key",
				Description: "Create a new API key",
				OperationID: "createAPIKey",
				Security:    jwtSecurity,
				RequestBody: &RequestBody{
					Required: true,
					Content: map[string]MediaType{
						"application/json": {
							Schema: &Schema{
								Type: "object",
								Properties: map[string]*Schema{
									"name":       {Type: "string"},
									"scopes":     {Type: "array", Items: &Schema{Type: "string"}},
									"expires_in": {Type: "integer", Description: "Days until expiration"},
								},
								Required: []string{"name"},
							},
						},
					},
				},
				Responses: map[string]Response{
					"201": {Description: "API key created"},
				},
			},
		},
		"/v1/contracts": {
			Get: &Operation{
				Tags:        []string{"contract"},
				Summary:     "List contracts",
				Description: "List registered smart contracts",
				OperationID: "listContracts",
				Security:    jwtSecurity,
				Responses: map[string]Response{
					"200": {
						Description: "Contracts listed",
						Content: map[string]MediaType{
							"application/json": {Schema: &Schema{Type: "array", Items: &Schema{Ref: "#/components/schemas/Contract"}}},
						},
					},
				},
			},
			Post: &Operation{
				Tags:        []string{"contract"},
				Summary:     "Register contract",
				Description: "Register a smart contract for monitoring",
				OperationID: "registerContract",
				Security:    jwtSecurity,
				RequestBody: &RequestBody{
					Required: true,
					Content: map[string]MediaType{
						"application/json": {
							Schema: &Schema{
								Type: "object",
								Properties: map[string]*Schema{
									"address":       {Type: "string"},
									"name":          {Type: "string"},
									"contract_type": {Type: "string"},
								},
								Required: []string{"address"},
							},
						},
					},
				},
				Responses: map[string]Response{
					"201": {
						Description: "Contract registered",
						Content: map[string]MediaType{
							"application/json": {Schema: &Schema{Ref: "#/components/schemas/Contract"}},
						},
					},
				},
			},
		},
		"/v1/multisig": {
			Get: &Operation{
				Tags:        []string{"multisig"},
				Summary:     "List multisig wallets",
				Description: "List multi-signature wallets",
				OperationID: "listMultisigWallets",
				Security:    jwtSecurity,
				Responses: map[string]Response{
					"200": {
						Description: "Wallets listed",
						Content: map[string]MediaType{
							"application/json": {Schema: &Schema{Type: "array", Items: &Schema{Ref: "#/components/schemas/MultisigWallet"}}},
						},
					},
				},
			},
		},
		"/v1/tx/batch": {
			Get: &Operation{
				Tags:        []string{"batch"},
				Summary:     "List batches",
				Description: "List transaction batches",
				OperationID: "listBatches",
				Security:    jwtSecurity,
				Responses: map[string]Response{
					"200": {
						Description: "Batches listed",
						Content: map[string]MediaType{
							"application/json": {Schema: &Schema{Type: "array", Items: &Schema{Ref: "#/components/schemas/Batch"}}},
						},
					},
				},
			},
			Post: &Operation{
				Tags:        []string{"batch"},
				Summary:     "Create batch",
				Description: "Create a new transaction batch",
				OperationID: "createBatch",
				Security:    jwtSecurity,
				RequestBody: &RequestBody{
					Required: true,
					Content: map[string]MediaType{
						"application/json": {
							Schema: &Schema{
								Type: "object",
								Properties: map[string]*Schema{
									"transactions": {
										Type:  "array",
										Items: &Schema{Ref: "#/components/schemas/SendTxRequest"},
									},
								},
								Required: []string{"transactions"},
							},
						},
					},
				},
				Responses: map[string]Response{
					"201": {
						Description: "Batch created",
						Content: map[string]MediaType{
							"application/json": {Schema: &Schema{Ref: "#/components/schemas/Batch"}},
						},
					},
				},
			},
		},
		"/auth/2fa/setup": {
			Post: &Operation{
				Tags:        []string{"2fa"},
				Summary:     "Setup 2FA",
				Description: "Initialize TOTP two-factor authentication",
				OperationID: "setup2FA",
				Security:    jwtSecurity,
				Responses: map[string]Response{
					"200": {
						Description: "2FA setup initiated",
						Content: map[string]MediaType{
							"application/json": {
								Schema: &Schema{
									Type: "object",
									Properties: map[string]*Schema{
										"secret":       {Type: "string"},
										"qr_code":      {Type: "string"},
										"backup_codes": {Type: "array", Items: &Schema{Type: "string"}},
									},
								},
							},
						},
					},
				},
			},
		},
		"/auth/2fa/verify": {
			Post: &Operation{
				Tags:        []string{"2fa"},
				Summary:     "Verify 2FA",
				Description: "Verify and enable TOTP two-factor authentication",
				OperationID: "verify2FA",
				Security:    jwtSecurity,
				RequestBody: &RequestBody{
					Required: true,
					Content: map[string]MediaType{
						"application/json": {
							Schema: &Schema{
								Type: "object",
								Properties: map[string]*Schema{
									"code": {Type: "string"},
								},
								Required: []string{"code"},
							},
						},
					},
				},
				Responses: map[string]Response{
					"200": {Description: "2FA enabled"},
					"400": {Description: "Invalid code"},
				},
			},
		},
	}
}

// WriteSpec writes the generated spec to a JSON encoder.
func WriteSpec(spec *Spec) ([]byte, error) {
	return json.MarshalIndent(spec, "", "  ")
}
