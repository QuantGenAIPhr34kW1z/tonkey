package tg

import (
	"context"
	"fmt"
	"strings"
	"time"

	tgbot "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"tonkey/internal/store"
	"tonkey/internal/ton"
)

type Bot struct {
	API        *tgbot.BotAPI
	AllowChats map[int64]bool
	Store      *store.Store
	Provider   ton.Provider
}

func (b *Bot) allowed(chatID int64) bool {
	if len(b.AllowChats) == 0 {
		return true
	}
	return b.AllowChats[chatID]
}

func (b *Bot) RunPoll(ctx context.Context) error {
	u := tgbot.NewUpdate(0)
	u.Timeout = 30

	updates := b.API.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			return nil
		case up := <-updates:
			if up.Message == nil {
				continue
			}
			chatID := up.Message.Chat.ID
			if !b.allowed(chatID) {
				continue
			}
			txt := strings.TrimSpace(up.Message.Text)
			if txt == "" {
				continue
			}
			resp := b.handle(chatID, txt)
			msg := tgbot.NewMessage(chatID, resp)
			_, _ = b.API.Send(msg)
		}
	}
}

func (b *Bot) handle(chatID int64, txt string) string {
	parts := strings.Fields(txt)
	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "/balance":
		if len(parts) != 2 {
			return "usage: /balance <addr>"
		}
		addr := parts[1]
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		bal, err := b.Provider.Balance(ctx, addr)
		if err != nil {
			return "provider error"
		}
		return fmt.Sprintf("balance(%s) = %d", addr, bal)

	case "/watch":
		if len(parts) != 2 {
			return "usage: /watch <addr>"
		}
		if err := b.Store.AddWatch(chatID, parts[1]); err != nil {
			return "store error"
		}
		return "ok"

	case "/unwatch":
		if len(parts) != 2 {
			return "usage: /unwatch <addr>"
		}
		if err := b.Store.RemoveWatch(chatID, parts[1]); err != nil {
			return "store error"
		}
		return "ok"

	case "/alerts", "/watchlist":
		addrs, err := b.Store.ListWatch(chatID)
		if err != nil {
			return "store error"
		}
		if len(addrs) == 0 {
			return "watchlist empty"
		}
		return "watching:\n- " + strings.Join(addrs, "\n- ")

	default:
		return "commands: /balance /watch /unwatch /alerts"
	}
}
