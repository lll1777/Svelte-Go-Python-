import { writable, get } from 'svelte/store';
import { offlineStorage } from '$lib/utils/offlineStorage';
import { apiClient } from '$lib/api/client';

class WebSocketService {
    constructor() {
        this.ws = null;
        this.isConnected = writable(false);
        this.messages = writable([]);
        this.currentWorkOrderId = null;
        this.reconnectAttempts = 0;
        this.maxReconnectAttempts = 5;
        this.reconnectDelay = 3000;
    }

    connect() {
        if (this.ws?.readyState === WebSocket.OPEN) {
            return;
        }

        const token = apiClient.getToken();
        if (!token) {
            console.error('No token available for WebSocket connection');
            return;
        }

        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws`;

        try {
            this.ws = new WebSocket(wsUrl);
            
            this.ws.onopen = () => {
                console.log('WebSocket connected');
                this.isConnected.set(true);
                this.reconnectAttempts = 0;
                
                this.send({
                    type: 'auth',
                    token
                });
            };

            this.ws.onmessage = (event) => {
                try {
                    const message = JSON.parse(event.data);
                    this.handleMessage(message);
                } catch (error) {
                    console.error('Failed to parse WebSocket message:', error);
                }
            };

            this.ws.onclose = (event) => {
                console.log('WebSocket closed:', event.code, event.reason);
                this.isConnected.set(false);
                this.attemptReconnect();
            };

            this.ws.onerror = (error) => {
                console.error('WebSocket error:', error);
            };
        } catch (error) {
            console.error('Failed to create WebSocket:', error);
            this.attemptReconnect();
        }
    }

    disconnect() {
        if (this.ws) {
            this.ws.close();
            this.ws = null;
        }
        this.isConnected.set(false);
    }

    attemptReconnect() {
        if (this.reconnectAttempts >= this.maxReconnectAttempts) {
            console.error('Max reconnect attempts reached');
            return;
        }

        this.reconnectAttempts++;
        console.log(`Attempting to reconnect (${this.reconnectAttempts}/${this.maxReconnectAttempts})...`);

        setTimeout(() => {
            this.connect();
        }, this.reconnectDelay * this.reconnectAttempts);
    }

    send(data) {
        if (this.ws?.readyState === WebSocket.OPEN) {
            this.ws.send(JSON.stringify(data));
        } else {
            console.error('WebSocket is not connected');
        }
    }

    handleMessage(message) {
        const { type, payload } = message;

        switch (type) {
            case 'new_message':
                this.handleNewMessage(payload);
                break;

            case 'status_update':
                this.handleStatusUpdate(payload);
                break;

            case 'new_work_order':
                this.handleNewWorkOrder(payload);
                break;

            case 'new_prescription':
                this.handleNewPrescription(payload);
                break;

            case 'typing':
                this.handleTyping(payload);
                break;

            case 'history':
                this.handleHistory(payload);
                break;

            case 'pong':
                break;

            default:
                console.log('Unknown message type:', type);
        }
    }

    async handleNewMessage(message) {
        if (message.work_order_id === this.currentWorkOrderId) {
            this.messages.update(msgs => [...msgs, message]);
        }

        await offlineStorage.saveMessage(message);
    }

    handleStatusUpdate(payload) {
        console.log('Status update:', payload);
    }

    handleNewWorkOrder(payload) {
        console.log('New work order:', payload);
    }

    handleNewPrescription(payload) {
        console.log('New prescription:', payload);
    }

    handleTyping(payload) {
        console.log('User typing:', payload);
    }

    handleHistory(messages) {
        this.messages.set(messages);
    }

    joinWorkOrder(workOrderId) {
        this.currentWorkOrderId = workOrderId;
        this.send({
            type: 'join',
            work_order_id: workOrderId
        });

        this.loadMessages(workOrderId);
    }

    leaveWorkOrder(workOrderId) {
        this.send({
            type: 'leave',
            work_order_id: workOrderId
        });
        
        if (this.currentWorkOrderId === workOrderId) {
            this.currentWorkOrderId = null;
        }
    }

    async sendMessage(workOrderId, content, imageUrl = null) {
        const message = {
            type: 'message',
            work_order_id: workOrderId,
            content,
            image_url: imageUrl
        };

        if (this.ws?.readyState === WebSocket.OPEN) {
            this.send(message);
        } else {
            const localMessage = {
                id: `local_${Date.now()}`,
                work_order_id: workOrderId,
                sender_id: 'current_user',
                message_type: imageUrl ? 'image' : 'text',
                content,
                image_url: imageUrl,
                is_read: false,
                created_at: new Date().toISOString(),
                isOffline: true
            };

            await offlineStorage.saveMessage(localMessage);
            await offlineStorage.addToSyncQueue('message', 'send', {
                workOrderId,
                content,
                imageUrl
            });

            this.messages.update(msgs => [...msgs, localMessage]);
        }
    }

    async loadMessages(workOrderId, limit = 50) {
        try {
            if (navigator.onLine) {
                const response = await apiClient.get(`/messages/${workOrderId}?limit=${limit}`);
                const messages = response.messages || [];
                this.messages.set(messages);

                for (const msg of messages) {
                    await offlineStorage.saveMessage(msg);
                }
            } else {
                const messages = await offlineStorage.getMessages(workOrderId, limit);
                this.messages.set(messages);
            }
        } catch (error) {
            console.error('Failed to load messages:', error);
            const messages = await offlineStorage.getMessages(workOrderId, limit);
            this.messages.set(messages);
        }
    }

    startTyping(workOrderId) {
        this.send({
            type: 'typing',
            work_order_id: workOrderId
        });
    }

    ping() {
        this.send({ type: 'ping' });
    }
}

export const wsService = new WebSocketService();
