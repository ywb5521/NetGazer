package models

import "encoding/json"

type NotificationChannelType string

const (
	ChannelGenericWebhook NotificationChannelType = "generic_webhook"
	ChannelSlack          NotificationChannelType = "slack"
	ChannelDingTalk       NotificationChannelType = "dingtalk"
	ChannelFeishu         NotificationChannelType = "feishu"
	ChannelEmail          NotificationChannelType = "email"
	ChannelTelegram       NotificationChannelType = "telegram"
)

type NotificationChannel struct {
	ID      string                  `json:"id"`
	Name    string                  `json:"name"`
	Type    NotificationChannelType `json:"type"`
	Enabled bool                    `json:"enabled"`
	Config  json.RawMessage         `json:"config"`
}
