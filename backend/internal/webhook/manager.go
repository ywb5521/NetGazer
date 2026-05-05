package webhook

import (
	"log"
	"sync"

	"github.com/netgazer/backend/internal/models"
)

type Manager struct {
	mu       sync.RWMutex
	channels []models.NotificationChannel
	senders  []ChannelSender
}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) SetChannels(channels []models.NotificationChannel) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.channels = channels
	m.senders = nil
	for _, ch := range channels {
		if !ch.Enabled {
			continue
		}
		sender, err := NewSender(ch)
		if err != nil {
			log.Printf("[notif] failed to create sender for channel %s (%s): %v", ch.Name, ch.Type, err)
			continue
		}
		m.senders = append(m.senders, sender)
	}
	log.Printf("[notif] loaded %d senders from %d channels", len(m.senders), len(channels))
}

func (m *Manager) GetChannels() []models.NotificationChannel {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.channels
}

func (m *Manager) Send(alert models.Alert) {
	m.mu.RLock()
	senders := m.senders
	m.mu.RUnlock()

	for _, s := range senders {
		go func(sender ChannelSender) {
			if err := sender.Send(alert); err != nil {
				log.Printf("[notif] %s send failed: %v", sender.Type(), err)
			} else {
				log.Printf("[notif] %s sent alert %s", sender.Type(), alert.ID)
			}
		}(s)
	}
}

func (m *Manager) Test(channelID string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, ch := range m.channels {
		if ch.ID == channelID {
			return SendTest(ch)
		}
	}
	return nil
}
