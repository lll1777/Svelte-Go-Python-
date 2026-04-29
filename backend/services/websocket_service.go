package services

import (
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"agriculture-platform/database"
	"agriculture-platform/models"
)

type WebSocketService struct {
	clients    map[string]*websocket.Conn
	workOrderClients map[string]map[string]bool
	mu         sync.RWMutex
}

type WebSocketMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

var webSocketService *WebSocketService

func GetWebSocketService() *WebSocketService {
	if webSocketService == nil {
		webSocketService = &WebSocketService{
			clients:           make(map[string]*websocket.Conn),
			workOrderClients:  make(map[string]map[string]bool),
		}
	}
	return webSocketService
}

func (s *WebSocketService) AddClient(userID string, conn *websocket.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if oldConn, exists := s.clients[userID]; exists {
		oldConn.Close()
	}

	s.clients[userID] = conn

	go s.heartbeat(userID, conn)
}

func (s *WebSocketService) RemoveClient(userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if conn, exists := s.clients[userID]; exists {
		conn.Close()
		delete(s.clients, userID)
	}

	for workOrderID, clients := range s.workOrderClients {
		delete(clients, userID)
		if len(clients) == 0 {
			delete(s.workOrderClients, workOrderID)
		}
	}
}

func (s *WebSocketService) JoinWorkOrder(userID string, workOrderID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.workOrderClients[workOrderID]; !exists {
		s.workOrderClients[workOrderID] = make(map[string]bool)
	}
	s.workOrderClients[workOrderID][userID] = true
}

func (s *WebSocketService) LeaveWorkOrder(userID string, workOrderID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if clients, exists := s.workOrderClients[workOrderID]; exists {
		delete(clients, userID)
		if len(clients) == 0 {
			delete(s.workOrderClients, workOrderID)
		}
	}
}

func (s *WebSocketService) SendToUser(userID string, messageType string, payload interface{}) {
	s.mu.RLock()
	conn, exists := s.clients[userID]
	s.mu.RUnlock()

	if !exists {
		return
	}

	message := WebSocketMessage{
		Type:    messageType,
		Payload: payload,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := conn.WriteJSON(message); err != nil {
		log.Printf("Failed to send message to user %s: %v", userID, err)
		s.RemoveClient(userID)
	}
}

func (s *WebSocketService) BroadcastToWorkOrder(workOrderID string, excludeUserID string, messageType string, payload interface{}) {
	s.mu.RLock()
	clients, exists := s.workOrderClients[workOrderID]
	s.mu.RUnlock()

	if !exists {
		return
	}

	message := WebSocketMessage{
		Type:    messageType,
		Payload: payload,
	}

	for userID := range clients {
		if userID == excludeUserID {
			continue
		}

		s.mu.RLock()
		conn, connExists := s.clients[userID]
		s.mu.RUnlock()

		if connExists {
			s.mu.Lock()
			if err := conn.WriteJSON(message); err != nil {
				log.Printf("Failed to send message to user %s: %v", userID, err)
				s.RemoveClient(userID)
			}
			s.mu.Unlock()
		}
	}
}

func (s *WebSocketService) SendNewWorkOrderNotification(expertID string, workOrder *models.WorkOrder) {
	payload := map[string]interface{}{
		"work_order_id": workOrder.ID,
		"title":         workOrder.Title,
		"crop_type":     workOrder.CropType,
		"farmer_name":   workOrder.Farmer.FullName,
		"created_at":    workOrder.CreatedAt,
	}
	s.SendToUser(expertID, "new_work_order", payload)
}

func (s *WebSocketService) SendStatusUpdate(workOrderID string, newStatus models.WorkOrderStatus, reason string) {
	payload := map[string]interface{}{
		"work_order_id": workOrderID,
		"new_status":    newStatus,
		"reason":        reason,
		"timestamp":     time.Now(),
	}
	s.BroadcastToWorkOrder(workOrderID, "", "status_update", payload)
}

func (s *WebSocketService) SendNewMessage(workOrderID string, message *models.Message) {
	payload := map[string]interface{}{
		"id":           message.ID,
		"work_order_id": message.WorkOrderID,
		"sender_id":    message.SenderID,
		"message_type": message.MessageType,
		"content":      message.Content,
		"image_url":    message.ImageURL,
		"created_at":   message.CreatedAt,
	}
	s.BroadcastToWorkOrder(workOrderID, message.SenderID, "new_message", payload)
}

func (s *WebSocketService) SendPrescriptionNotification(workOrderID string, farmerID string, prescription *models.Prescription) {
	payload := map[string]interface{}{
		"work_order_id": workOrderID,
		"prescription_id": prescription.ID,
		"diagnosis":     prescription.Diagnosis,
		"created_at":    prescription.CreatedAt,
	}
	s.SendToUser(farmerID, "new_prescription", payload)
}

func (s *WebSocketService) heartbeat(userID string, conn *websocket.Conn) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
			err := conn.WriteMessage(websocket.PingMessage, nil)
			s.mu.Unlock()
			if err != nil {
				log.Printf("Heartbeat failed for user %s: %v", userID, err)
				s.RemoveClient(userID)
				return
			}
		}
	}
}

func (s *WebSocketService) SaveMessage(workOrderID string, senderID string, messageType models.MessageType, content string, imageURL *string) (*models.Message, error) {
	message := &models.Message{
		WorkOrderID: workOrderID,
		SenderID:    senderID,
		MessageType: messageType,
		Content:     content,
		ImageURL:    imageURL,
		IsRead:      false,
		CreatedAt:   time.Now(),
	}

	db := database.GetDB()
	if err := db.Create(message).Error; err != nil {
		return nil, err
	}

	return message, nil
}

func (s *WebSocketService) GetWorkOrderMessages(workOrderID string, limit int) ([]models.Message, error) {
	var messages []models.Message
	db := database.GetDB()

	if err := db.Where("work_order_id = ?", workOrderID).
		Order("created_at DESC").
		Limit(limit).
		Preload("Sender").
		Find(&messages).Error; err != nil {
		return nil, err
	}

	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}
