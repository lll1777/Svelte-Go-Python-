package controllers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"agriculture-platform/middleware"
	"agriculture-platform/models"
	"agriculture-platform/services"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type WebSocketController struct {
	wsService *services.WebSocketService
}

func NewWebSocketController() *WebSocketController {
	return &WebSocketController{
		wsService: services.GetWebSocketService(),
	}
}

type IncomingMessage struct {
	Type        string          `json:"type"`
	WorkOrderID string          `json:"work_order_id,omitempty"`
	Content     string          `json:"content,omitempty"`
	ImageURL    *string         `json:"image_url,omitempty"`
	Payload     json.RawMessage `json:"payload,omitempty"`
}

func (c *WebSocketController) HandleWebSocket(ctx *gin.Context) {
	userID := middleware.GetCurrentUserID(ctx)
	if userID == "" {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade websocket: %v", err)
		return
	}

	c.wsService.AddClient(userID, conn)
	defer c.wsService.RemoveClient(userID)

	log.Printf("User %s connected via WebSocket", userID)

	for {
		_, messageData, err := conn.ReadMessage()
		if err != nil {
			log.Printf("WebSocket read error for user %s: %v", userID, err)
			break
		}

		var msg IncomingMessage
		if err := json.Unmarshal(messageData, &msg); err != nil {
			log.Printf("Invalid message from user %s: %v", userID, err)
			continue
		}

		c.handleMessage(userID, &msg)
	}
}

func (c *WebSocketController) handleMessage(userID string, msg *IncomingMessage) {
	switch msg.Type {
	case "join":
		if msg.WorkOrderID != "" {
			c.wsService.JoinWorkOrder(userID, msg.WorkOrderID)
			log.Printf("User %s joined work order %s", userID, msg.WorkOrderID)

			messages, err := c.wsService.GetWorkOrderMessages(msg.WorkOrderID, 50)
			if err == nil {
				c.wsService.SendToUser(userID, "history", messages)
			}
		}

	case "leave":
		if msg.WorkOrderID != "" {
			c.wsService.LeaveWorkOrder(userID, msg.WorkOrderID)
			log.Printf("User %s left work order %s", userID, msg.WorkOrderID)
		}

	case "message":
		if msg.WorkOrderID != "" {
			messageType := models.MessageTypeText
			if msg.ImageURL != nil {
				messageType = models.MessageTypeImage
			}

			message, err := c.wsService.SaveMessage(
				msg.WorkOrderID,
				userID,
				messageType,
				msg.Content,
				msg.ImageURL,
			)

			if err == nil {
				c.wsService.SendNewMessage(msg.WorkOrderID, message)
			}
		}

	case "typing":
		if msg.WorkOrderID != "" {
			c.wsService.BroadcastToWorkOrder(
				msg.WorkOrderID,
				userID,
				"typing",
				gin.H{"user_id": userID, "is_typing": true},
			)
		}

	case "ping":
		c.wsService.SendToUser(userID, "pong", nil)
	}
}

func (c *WebSocketController) GetMessages(ctx *gin.Context) {
	workOrderID := ctx.Param("id")
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "50"))

	userID := middleware.GetCurrentUserID(ctx)

	workOrderService := services.NewWorkOrderService()
	wo, err := workOrderService.GetWorkOrderByID(workOrderID)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Work order not found"})
		return
	}

	userRole := middleware.GetCurrentUserRole(ctx)
	if userRole != string(models.RoleAdmin) {
		if wo.FarmerID != userID && (wo.ExpertID == nil || *wo.ExpertID != userID) {
			ctx.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}
	}

	messages, err := c.wsService.GetWorkOrderMessages(workOrderID, limit)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"messages": messages,
		"count":    len(messages),
	})
}
