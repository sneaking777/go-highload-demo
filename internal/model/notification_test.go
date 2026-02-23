package model

import (
	"testing"
	"time"
)

func TestNotificationStatus_IsValid(t *testing.T) {
	tests := []struct {
		name   string
		status NotificationStatus
		want   bool
	}{
		{"pending is valid", StatusPending, true},
		{"processing is valid", StatusProcessing, true},
		{"sent is valid", StatusSent, true},
		{"failed is valid", StatusFailed, true},
		{"empty is invalid", NotificationStatus(""), false},
		{"unknown is invalid", NotificationStatus("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.want {
				t.Errorf("NotificationStatus(%q).IsValid() = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestChannel_IsValid(t *testing.T) {
	tests := []struct {
		name    string
		channel Channel
		want    bool
	}{
		{"email is valid", ChannelEmail, true},
		{"push is valid", ChannelPush, true},
		{"sms is valid", ChannelSMS, true},
		{"webhook is valid", ChannelWebhook, true},
		{"empty is invalid", Channel(""), false},
		{"unknown is invalid", Channel("telegram"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.channel.IsValid(); got != tt.want {
				t.Errorf("Channel(%q).IsValid() = %v, want %v", tt.channel, got, tt.want)
			}
		})
	}
}

func TestNotification_Validate(t *testing.T) {
	valid := Notification{
		UserID:  "user-123",
		Channel: ChannelEmail,
		Payload: `{"to": "test@example.com", "subject": "Hello", "body": "World"}`,
	}

	t.Run("valid notification", func(t *testing.T) {
		if err := valid.Validate(); err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("empty user id", func(t *testing.T) {
		n := valid
		n.UserID = ""
		if err := n.Validate(); err == nil {
			t.Error("expected error for empty UserID")
		}
	})

	t.Run("invalid channel", func(t *testing.T) {
		n := valid
		n.Channel = "pigeon"
		if err := n.Validate(); err == nil {
			t.Error("expected error for invalid channel")
		}
	})

	t.Run("empty payload", func(t *testing.T) {
		n := valid
		n.Payload = ""
		if err := n.Validate(); err == nil {
			t.Error("expected error for empty payload")
		}
	})
}

func TestNotification_NewNotification(t *testing.T) {
	n := NewNotification("user-1", ChannelSMS, `{"phone":"+7999"}`)

	if n.ID == "" {
		t.Error("expected ID to be generated")
	}
	if n.Status != StatusPending {
		t.Errorf("expected status %q, got %q", StatusPending, n.Status)
	}
	if n.RetryCount != 0 {
		t.Errorf("expected retry count 0, got %d", n.RetryCount)
	}
	if n.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
	if n.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
}

func TestNotification_MarkProcessing(t *testing.T) {
	n := NewNotification("user-1", ChannelEmail, `{"to":"a@b.com"}`)
	before := n.UpdatedAt

	time.Sleep(time.Millisecond)
	n.MarkProcessing()

	if n.Status != StatusProcessing {
		t.Errorf("expected status %q, got %q", StatusProcessing, n.Status)
	}
	if !n.UpdatedAt.After(before) {
		t.Error("expected UpdatedAt to be updated")
	}
}

func TestNotification_MarkSent(t *testing.T) {
	n := NewNotification("user-1", ChannelPush, `{"token":"abc"}`)
	n.MarkSent()

	if n.Status != StatusSent {
		t.Errorf("expected status %q, got %q", StatusSent, n.Status)
	}
}

func TestNotification_MarkFailed(t *testing.T) {
	n := NewNotification("user-1", ChannelWebhook, `{"url":"http://example.com"}`)
	n.MarkFailed("connection timeout")

	if n.Status != StatusFailed {
		t.Errorf("expected status %q, got %q", StatusFailed, n.Status)
	}
	if n.LastError != "connection timeout" {
		t.Errorf("expected error %q, got %q", "connection timeout", n.LastError)
	}
	if n.RetryCount != 1 {
		t.Errorf("expected retry count 1, got %d", n.RetryCount)
	}

	n.MarkFailed("timeout again")
	if n.RetryCount != 2 {
		t.Errorf("expected retry count 2, got %d", n.RetryCount)
	}
}
