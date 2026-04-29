package models

import (
	"time"

	"github.com/google/uuid"
)

type MessageType string

const (
	MessageTypeText       MessageType = "text"
	MessageTypeImage      MessageType = "image"
	MessageTypePrescription MessageType = "prescription"
	MessageTypeSystem     MessageType = "system"
)

type Message struct {
	ID          string      `gorm:"primary_key;type:varchar(36)" json:"id"`
	WorkOrderID string      `gorm:"type:varchar(36);index" json:"work_order_id"`
	SenderID    string      `gorm:"type:varchar(36);index" json:"sender_id"`
	ReceiverID  *string     `gorm:"type:varchar(36);index" json:"receiver_id,omitempty"`
	MessageType MessageType `gorm:"type:varchar(20)" json:"message_type"`
	Content     string      `gorm:"type:text" json:"content"`
	ImageURL    *string     `gorm:"type:varchar(500)" json:"image_url,omitempty"`
	IsRead      bool        `gorm:"default:false" json:"is_read"`
	ReadAt      *time.Time  `json:"read_at,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`

	Sender *User `gorm:"foreignkey:SenderID" json:"sender,omitempty"`
}

type Notification struct {
	ID          string    `gorm:"primary_key;type:varchar(36)" json:"id"`
	UserID      string    `gorm:"type:varchar(36);index" json:"user_id"`
	Title       string    `gorm:"type:varchar(200)" json:"title"`
	Content     string    `gorm:"type:text" json:"content"`
	WorkOrderID *string   `gorm:"type:varchar(36);index" json:"work_order_id,omitempty"`
	IsRead      bool      `gorm:"default:false" json:"is_read"`
	ReadAt      *time.Time `json:"read_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`

	User *User `gorm:"foreignkey:UserID" json:"user,omitempty"`
}

func (m *Message) BeforeCreate() (err error) {
	m.ID = uuid.New().String()
	return nil
}

func (n *Notification) BeforeCreate() (err error) {
	n.ID = uuid.New().String()
	return nil
}
