// Package main provides a CLI client for tonkey API.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

var (
	apiURL string
	token  string
)

func init() {
	// Load from environment
	apiURL = os.Getenv("TONKEY_API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:8080"
	}
	token = os.Getenv("TONKEY_TOKEN")

	// Try to load token from config file
	if token == "" {
		token = loadToken()
	}
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "login":
		loginCmd()
	case "balance":
		balanceCmd()
	case "send":
		sendCmd()
	case "tx":
		txCmd()
	case "history":
		historyCmd()
	case "webhooks":
		webhooksCmd()
	case "apikeys":
		apikeysCmd()
	case "help":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("tonkey CLI - TON Blockchain Gateway Client")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  tonkey-cli <command> [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  login     Authenticate with tonkey")
	fmt.Println("  balance   Get wallet balance")
	fmt.Println("  send      Send a transaction")
	fmt.Println("  tx        Get transaction details")
	fmt.Println("  history   List transactions")
	fmt.Println("  webhooks  Manage webhooks")
	fmt.Println("  apikeys   Manage API keys")
	fmt.Println("  help      Show this help")
	fmt.Println()
	fmt.Println("Environment:")
	fmt.Println("  TONKEY_API_URL  API URL (default: http://localhost:8080)")
	fmt.Println("  TONKEY_TOKEN    JWT token")
}

func loginCmd() {
	fs := flag.NewFlagSet("login", flag.ExitOnError)
	user := fs.String("u", "", "Username")
	pass := fs.String("p", "", "Password")
	fs.Parse(os.Args[2:])

	if *user == "" || *pass == "" {
		fmt.Println("Usage: tonkey-cli login -u <username> -p <password>")
		os.Exit(1)
	}

	body := map[string]string{"user": *user, "pass": *pass}
	resp, err := doRequest("POST", "/auth/login", body, false)
	if err != nil {
		fmt.Printf("Login failed: %v\n", err)
		os.Exit(1)
	}

	var result struct {
		Token string `json:"token"`
	}
	json.Unmarshal(resp, &result)

	if result.Token == "" {
		fmt.Println("Login failed: no token received")
		os.Exit(1)
	}

	saveToken(result.Token)
	fmt.Println("Login successful. Token saved.")
}

func balanceCmd() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: tonkey-cli balance <address>")
		os.Exit(1)
	}

	address := os.Args[2]
	resp, err := doRequest("GET", "/v1/wallet/"+address+"/balance", nil, true)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	var result struct {
		Address string `json:"address"`
		Balance int64  `json:"balance"`
	}
	json.Unmarshal(resp, &result)

	fmt.Printf("Address: %s\n", result.Address)
	fmt.Printf("Balance: %.4f TON\n", float64(result.Balance)/1e9)
}

func sendCmd() {
	fs := flag.NewFlagSet("send", flag.ExitOnError)
	from := fs.String("from", "", "Sender address")
	to := fs.String("to", "", "Recipient address")
	amount := fs.Int64("amount", 0, "Amount in nanoTON")
	fs.Parse(os.Args[2:])

	if *from == "" || *to == "" || *amount <= 0 {
		fmt.Println("Usage: tonkey-cli send --from <addr> --to <addr> --amount <nanoTON>")
		os.Exit(1)
	}

	body := map[string]any{"from": *from, "to": *to, "amount": *amount}
	resp, err := doRequest("POST", "/v1/tx/send", body, true)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	var result struct {
		ID string `json:"id"`
	}
	json.Unmarshal(resp, &result)

	fmt.Printf("Transaction submitted. ID: %s\n", result.ID)
}

func txCmd() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: tonkey-cli tx <id>")
		os.Exit(1)
	}

	id := os.Args[2]
	resp, err := doRequest("GET", "/v1/tx/"+id, nil, true)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	var tx struct {
		ID        string `json:"id"`
		From      string `json:"from"`
		To        string `json:"to"`
		Amount    int64  `json:"amount"`
		Status    string `json:"status"`
		CreatedAt int64  `json:"created_at"`
	}
	json.Unmarshal(resp, &tx)

	fmt.Printf("ID:        %s\n", tx.ID)
	fmt.Printf("From:      %s\n", tx.From)
	fmt.Printf("To:        %s\n", tx.To)
	fmt.Printf("Amount:    %.4f TON\n", float64(tx.Amount)/1e9)
	fmt.Printf("Status:    %s\n", tx.Status)
	fmt.Printf("Created:   %s\n", time.Unix(tx.CreatedAt, 0).Format(time.RFC3339))
}

func historyCmd() {
	fs := flag.NewFlagSet("history", flag.ExitOnError)
	limit := fs.Int("limit", 10, "Number of transactions")
	status := fs.String("status", "", "Filter by status")
	fs.Parse(os.Args[2:])

	body := map[string]any{
		"entity": "tx",
		"limit":  *limit,
	}
	if *status != "" {
		body["filters"] = map[string]string{"status": *status}
	}

	resp, err := doRequest("POST", "/v1/query", body, true)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	var result struct {
		Count int `json:"count"`
		Rows  []struct {
			ID        string `json:"id"`
			From      string `json:"from"`
			To        string `json:"to"`
			Amount    int64  `json:"amount"`
			Status    string `json:"status"`
			CreatedAt int64  `json:"created_at"`
		} `json:"rows"`
	}
	json.Unmarshal(resp, &result)

	fmt.Printf("Found %d transactions:\n\n", result.Count)
	for _, tx := range result.Rows {
		fmt.Printf("ID: %s\n", tx.ID)
		fmt.Printf("  From:   %s\n", truncateAddr(tx.From))
		fmt.Printf("  To:     %s\n", truncateAddr(tx.To))
		fmt.Printf("  Amount: %.4f TON\n", float64(tx.Amount)/1e9)
		fmt.Printf("  Status: %s\n", tx.Status)
		fmt.Println()
	}
}

func webhooksCmd() {
	fs := flag.NewFlagSet("webhooks", flag.ExitOnError)
	action := fs.String("action", "list", "Action: list, create, delete")
	url := fs.String("url", "", "Webhook URL")
	eventType := fs.String("event", "transaction", "Event type")
	secret := fs.String("secret", "", "Webhook secret")
	id := fs.Int64("id", 0, "Webhook ID (for delete)")
	fs.Parse(os.Args[2:])

	switch *action {
	case "list":
		resp, err := doRequest("GET", "/v1/webhooks", nil, true)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(resp))

	case "create":
		if *url == "" || *secret == "" {
			fmt.Println("Usage: tonkey-cli webhooks -action create -url <url> -secret <secret>")
			os.Exit(1)
		}
		body := map[string]string{
			"url":        *url,
			"event_type": *eventType,
			"secret":     *secret,
		}
		resp, err := doRequest("POST", "/v1/webhooks", body, true)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Webhook created:")
		fmt.Println(string(resp))

	case "delete":
		if *id == 0 {
			fmt.Println("Usage: tonkey-cli webhooks -action delete -id <id>")
			os.Exit(1)
		}
		_, err := doRequest("DELETE", fmt.Sprintf("/v1/webhooks/%d", *id), nil, true)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Webhook deleted")
	}
}

func apikeysCmd() {
	fs := flag.NewFlagSet("apikeys", flag.ExitOnError)
	action := fs.String("action", "list", "Action: list, create, delete")
	name := fs.String("name", "", "Key name")
	id := fs.Int64("id", 0, "Key ID (for delete)")
	fs.Parse(os.Args[2:])

	switch *action {
	case "list":
		resp, err := doRequest("GET", "/v1/api-keys", nil, true)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(resp))

	case "create":
		if *name == "" {
			fmt.Println("Usage: tonkey-cli apikeys -action create -name <name>")
			os.Exit(1)
		}
		body := map[string]any{"name": *name}
		resp, err := doRequest("POST", "/v1/api-keys", body, true)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("API key created:")
		fmt.Println(string(resp))

	case "delete":
		if *id == 0 {
			fmt.Println("Usage: tonkey-cli apikeys -action delete -id <id>")
			os.Exit(1)
		}
		_, err := doRequest("DELETE", fmt.Sprintf("/v1/api-keys/%d", *id), nil, true)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("API key deleted")
	}
}

func doRequest(method, path string, body any, auth bool) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, apiURL+path, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if auth && token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("request failed (%d): %s", resp.StatusCode, string(data))
	}

	return data, nil
}

func loadToken() string {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".tonkey", "token")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(bytes.TrimSpace(data))
}

func saveToken(t string) {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".tonkey")
	os.MkdirAll(dir, 0700)
	path := filepath.Join(dir, "token")
	os.WriteFile(path, []byte(t), 0600)
}

func truncateAddr(addr string) string {
	if len(addr) <= 16 {
		return addr
	}
	return addr[:8] + "..." + addr[len(addr)-8:]
}
