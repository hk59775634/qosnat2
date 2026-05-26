package store

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

const maxUINotifications = 80

// UINotification 管理界面右上角消息（持久化，可多客户端轮询）
type UINotification struct {
	ID        string `json:"id"`
	Level     string `json:"level"` // info | success | warn | error
	Title     string `json:"title"`
	Message   string `json:"message"`
	Link      string `json:"link,omitempty"`
	CreatedAt string `json:"created_at"`
	Read      bool   `json:"read,omitempty"`
}

func NewNotificationID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "ntf-" + hex.EncodeToString(b)
}

// PrependNotification 追加通知并限制条数
func PrependNotification(list []UINotification, n UINotification) []UINotification {
	if n.ID == "" {
		n.ID = NewNotificationID()
	}
	if n.CreatedAt == "" {
		n.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	out := append([]UINotification{n}, list...)
	if len(out) > maxUINotifications {
		out = out[:maxUINotifications]
	}
	return out
}
