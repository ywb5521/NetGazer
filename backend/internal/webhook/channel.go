package webhook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"strings"
	"time"

	"github.com/netgazer/backend/internal/models"
)

type ChannelSender interface {
	Send(alert models.Alert) error
	Type() models.NotificationChannelType
}

// ---- generic_webhook ----

type GenericWebhookSender struct {
	url    string
	client *http.Client
}

func (s *GenericWebhookSender) Type() models.NotificationChannelType {
	return models.ChannelGenericWebhook
}

func (s *GenericWebhookSender) Send(alert models.Alert) error {
	payload := map[string]interface{}{
		"alert":     alert.ToJSON(),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"source":    "netgazer",
	}
	body, _ := json.Marshal(payload)
	resp, err := s.client.Post(s.url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

// ---- slack ----

type SlackSender struct {
	url    string
	client *http.Client
}

func (s *SlackSender) Type() models.NotificationChannelType { return models.ChannelSlack }

func (s *SlackSender) Send(alert models.Alert) error {
	color := "#ffcc00"
	switch alert.Severity {
	case models.SeverityCritical:
		color = "#ff0000"
	case models.SeverityWarning:
		color = "#ffcc00"
	case models.SeverityInfo:
		color = "#36a64f"
	}
	payload := map[string]interface{}{
		"attachments": []map[string]interface{}{{
			"color": color,
			"title": fmt.Sprintf("[%s] %s", alert.Type.LabelZH(), alert.Message),
			"fields": []map[string]interface{}{
				{"title": "级别", "value": alert.Severity.LabelZH(), "short": true},
				{"title": "源 IP", "value": alert.SourceIP, "short": true},
				{"title": "节点", "value": alert.NodeID, "short": true},
			},
			"ts": alert.Timestamp.Unix(),
		}},
	}
	body, _ := json.Marshal(payload)
	resp, err := s.client.Post(s.url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

// ---- dingtalk ----

type DingTalkSender struct {
	url    string
	client *http.Client
}

func (s *DingTalkSender) Type() models.NotificationChannelType { return models.ChannelDingTalk }

func (s *DingTalkSender) Send(alert models.Alert) error {
	title := fmt.Sprintf("netgazer 告警：%s", alert.Type.LabelZH())
	text := fmt.Sprintf("## %s\n\n**类型：** %s\n**级别：** %s\n**源地址：** %s\n**节点：** %s\n**时间：** %s\n\n%s",
		title, alert.Type.LabelZH(), alert.Severity.LabelZH(), alert.SourceIP,
		alert.NodeID, alert.Timestamp.Format(time.RFC3339), alert.Message)
	payload := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": title,
			"text":  text,
		},
	}
	body, _ := json.Marshal(payload)
	resp, err := s.client.Post(s.url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

// ---- feishu ----

type FeishuSender struct {
	url    string
	client *http.Client
}

func (s *FeishuSender) Type() models.NotificationChannelType { return models.ChannelFeishu }

func (s *FeishuSender) Send(alert models.Alert) error {
	color := "yellow"
	switch alert.Severity {
	case models.SeverityCritical:
		color = "red"
	case models.SeverityInfo:
		color = "green"
	}
	payload := map[string]interface{}{
		"msg_type": "interactive",
		"card": map[string]interface{}{
			"header": map[string]interface{}{
				"title": map[string]string{
					"tag":     "plain_text",
					"content": fmt.Sprintf("netgazer：%s", alert.Type.LabelZH()),
				},
				"template": color,
			},
			"elements": []map[string]interface{}{
				{"tag": "div", "text": map[string]string{"tag": "lark_md", "content": alert.Message}},
				{"tag": "div", "text": map[string]string{"tag": "lark_md", "content": fmt.Sprintf("**级别：** %s | **源地址：** %s | **节点：** %s", alert.Severity.LabelZH(), alert.SourceIP, alert.NodeID)}},
			},
		},
	}
	body, _ := json.Marshal(payload)
	resp, err := s.client.Post(s.url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

// ---- email ----

type EmailConfig struct {
	SMTPServer string   `json:"smtp_server"`
	SMTPPort   int      `json:"smtp_port"`
	Username   string   `json:"username"`
	Password   string   `json:"password"`
	From       string   `json:"from"`
	To         []string `json:"to"`
}

type EmailSender struct {
	cfg EmailConfig
}

func (s *EmailSender) Type() models.NotificationChannelType { return models.ChannelEmail }

func (s *EmailSender) Send(alert models.Alert) error {
	subject := fmt.Sprintf("netgazer [%s] %s - %s", alert.Severity.LabelZH(), alert.Type.LabelZH(), alert.SourceIP)
	body := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=\"UTF-8\"\r\n\r\n类型：%s\n级别：%s\n消息：%s\n源 IP：%s\n节点：%s\n时间：%s\n",
		s.cfg.From, strings.Join(s.cfg.To, ", "), subject,
		alert.Type.LabelZH(), alert.Severity.LabelZH(), alert.Message, alert.SourceIP,
		alert.NodeID, alert.Timestamp.Format(time.RFC3339))

	addr := fmt.Sprintf("%s:%d", s.cfg.SMTPServer, s.cfg.SMTPPort)
	auth := smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.SMTPServer)
	return smtp.SendMail(addr, auth, s.cfg.From, s.cfg.To, []byte(body))
}

// ---- telegram ----

type TelegramSender struct {
	apiURL string
	chatID string
	client *http.Client
}

func (s *TelegramSender) Type() models.NotificationChannelType { return models.ChannelTelegram }

func (s *TelegramSender) Send(alert models.Alert) error {
	emoji := "⚠️"
	switch alert.Severity {
	case models.SeverityCritical:
		emoji = "🔴"
	case models.SeverityWarning:
		emoji = "🟡"
	case models.SeverityInfo:
		emoji = "🟢"
	}
	text := fmt.Sprintf(
		"%s <b>[%s] %s</b>\n<pre>%s</pre>\n<b>源 IP：</b> %s\n<b>节点：</b> %s\n<b>时间：</b> %s",
		emoji, alert.Severity.LabelZH(), alert.Type.LabelZH(), alert.Message,
		alert.SourceIP, alert.NodeID, alert.Timestamp.Format(time.RFC3339),
	)
	payload := map[string]string{
		"chat_id":    s.chatID,
		"text":       text,
		"parse_mode": "HTML",
	}
	body, _ := json.Marshal(payload)
	resp, err := s.client.Post(s.apiURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

// ---- factory ----

func NewSender(ch models.NotificationChannel) (ChannelSender, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	switch ch.Type {
	case models.ChannelGenericWebhook:
		var cfg struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal(ch.Config, &cfg); err != nil {
			return nil, fmt.Errorf("invalid config: %w", err)
		}
		if cfg.URL == "" {
			return nil, fmt.Errorf("url is required")
		}
		return &GenericWebhookSender{url: cfg.URL, client: client}, nil

	case models.ChannelSlack:
		var cfg struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal(ch.Config, &cfg); err != nil {
			return nil, fmt.Errorf("invalid config: %w", err)
		}
		if cfg.URL == "" {
			return nil, fmt.Errorf("url is required")
		}
		return &SlackSender{url: cfg.URL, client: client}, nil

	case models.ChannelDingTalk:
		var cfg struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal(ch.Config, &cfg); err != nil {
			return nil, fmt.Errorf("invalid config: %w", err)
		}
		if cfg.URL == "" {
			return nil, fmt.Errorf("url is required")
		}
		return &DingTalkSender{url: cfg.URL, client: client}, nil

	case models.ChannelFeishu:
		var cfg struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal(ch.Config, &cfg); err != nil {
			return nil, fmt.Errorf("invalid config: %w", err)
		}
		if cfg.URL == "" {
			return nil, fmt.Errorf("url is required")
		}
		return &FeishuSender{url: cfg.URL, client: client}, nil

	case models.ChannelEmail:
		var cfg EmailConfig
		if err := json.Unmarshal(ch.Config, &cfg); err != nil {
			return nil, fmt.Errorf("invalid config: %w", err)
		}
		if cfg.SMTPServer == "" || len(cfg.To) == 0 {
			return nil, fmt.Errorf("smtp_server and to are required")
		}
		return &EmailSender{cfg: cfg}, nil

	case models.ChannelTelegram:
		var cfg struct {
			BotToken string `json:"bot_token"`
			ChatID   string `json:"chat_id"`
		}
		if err := json.Unmarshal(ch.Config, &cfg); err != nil {
			return nil, fmt.Errorf("invalid config: %w", err)
		}
		if cfg.BotToken == "" || cfg.ChatID == "" {
			return nil, fmt.Errorf("bot_token and chat_id are required")
		}
		return &TelegramSender{
			apiURL: fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", cfg.BotToken),
			chatID: cfg.ChatID,
			client: client,
		}, nil

	default:
		return nil, fmt.Errorf("unknown channel type: %s", ch.Type)
	}
}

// ---- test helpers ----

func SendTest(ch models.NotificationChannel) error {
	testAlert := models.Alert{
		ID:        "test-" + time.Now().Format("150405"),
		Type:      models.AlertType("test"),
		Severity:  models.SeverityInfo,
		Message:   "netgazer 测试通知",
		SourceIP:  "127.0.0.1",
		NodeID:    "test",
		Timestamp: time.Now(),
	}
	sender, err := NewSender(ch)
	if err != nil {
		return err
	}
	return sender.Send(testAlert)
}
