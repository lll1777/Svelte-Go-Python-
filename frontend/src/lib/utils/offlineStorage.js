import { openDB } from 'idb';

const DB_NAME = 'AgricultureOfflineDB';
const DB_VERSION = 1;

const STORES = {
    WORK_ORDERS: 'work_orders',
    MESSAGES: 'messages',
    DIAGNOSIS_RESULTS: 'diagnosis_results',
    IMAGES: 'images',
    SYNC_QUEUE: 'sync_queue',
    USER_DATA: 'user_data'
};

class OfflineStorage {
    constructor() {
        this.db = null;
        this.isOnline = navigator.onLine;
        this._initEventListeners();
    }

    async initDB() {
        if (this.db) {
            return this.db;
        }

        this.db = await openDB(DB_NAME, DB_VERSION, {
            upgrade(db) {
                if (!db.objectStoreNames.contains(STORES.WORK_ORDERS)) {
                    const workOrderStore = db.createObjectStore(STORES.WORK_ORDERS, { 
                        keyPath: 'id' 
                    });
                    workOrderStore.createIndex('farmerId', 'farmerId', { unique: false });
                    workOrderStore.createIndex('status', 'status', { unique: false });
                    workOrderStore.createIndex('createdAt', 'createdAt', { unique: false });
                }

                if (!db.objectStoreNames.contains(STORES.MESSAGES)) {
                    const messageStore = db.createObjectStore(STORES.MESSAGES, { 
                        keyPath: 'id' 
                    });
                    messageStore.createIndex('workOrderId', 'workOrderId', { unique: false });
                    messageStore.createIndex('createdAt', 'createdAt', { unique: false });
                }

                if (!db.objectStoreNames.contains(STORES.DIAGNOSIS_RESULTS)) {
                    const diagnosisStore = db.createObjectStore(STORES.DIAGNOSIS_RESULTS, { 
                        keyPath: 'imageHash' 
                    });
                    diagnosisStore.createIndex('workOrderId', 'workOrderId', { unique: false });
                }

                if (!db.objectStoreNames.contains(STORES.IMAGES)) {
                    const imageStore = db.createObjectStore(STORES.IMAGES, { 
                        keyPath: 'hash' 
                    });
                }

                if (!db.objectStoreNames.contains(STORES.SYNC_QUEUE)) {
                    const syncStore = db.createObjectStore(STORES.SYNC_QUEUE, { 
                        keyPath: 'id', 
                        autoIncrement: true 
                    });
                    syncStore.createIndex('type', 'type', { unique: false });
                    syncStore.createIndex('status', 'status', { unique: false });
                }

                if (!db.objectStoreNames.contains(STORES.USER_DATA)) {
                    db.createObjectStore(STORES.USER_DATA, { 
                        keyPath: 'key' 
                    });
                }
            }
        });

        return this.db;
    }

    _initEventListeners() {
        window.addEventListener('online', () => {
            this.isOnline = true;
            this._onOnline();
        });

        window.addEventListener('offline', () => {
            this.isOnline = false;
            this._onOffline();
        });
    }

    _onOnline() {
        console.log('Network is online, starting sync...');
        this.processSyncQueue();
    }

    _onOffline() {
        console.log('Network is offline');
    }

    async saveWorkOrder(workOrder) {
        const db = await this.initDB();
        const existing = await db.get(STORES.WORK_ORDERS, workOrder.id);
        
        if (existing) {
            await db.put(STORES.WORK_ORDERS, { ...existing, ...workOrder });
        } else {
            await db.add(STORES.WORK_ORDERS, workOrder);
        }
        
        if (workOrder.isOfflineCreated) {
            await this.addToSyncQueue('work_order', 'create', workOrder);
        }
        
        return workOrder;
    }

    async getWorkOrders(farmerId, status = null) {
        const db = await this.initDB();
        
        let workOrders;
        if (status) {
            const tx = db.transaction(STORES.WORK_ORDERS, 'readonly');
            const index = tx.store.index('status');
            workOrders = await index.getAll(status);
        } else {
            workOrders = await db.getAll(STORES.WORK_ORDERS);
        }

        if (farmerId) {
            workOrders = workOrders.filter(wo => wo.farmerId === farmerId);
        }

        return workOrders.sort((a, b) => new Date(b.createdAt) - new Date(a.createdAt));
    }

    async getWorkOrder(id) {
        const db = await this.initDB();
        return db.get(STORES.WORK_ORDERS, id);
    }

    async saveDiagnosisResult(diagnosis) {
        const db = await this.initDB();
        
        if (diagnosis.imageHash) {
            await db.put(STORES.DIAGNOSIS_RESULTS, diagnosis);
        }
        
        if (diagnosis.workOrderId) {
            const workOrder = await db.get(STORES.WORK_ORDERS, diagnosis.workOrderId);
            if (workOrder) {
                workOrder.diagnosisResult = diagnosis;
                await db.put(STORES.WORK_ORDERS, workOrder);
            }
        }
        
        return diagnosis;
    }

    async getDiagnosisByHash(imageHash) {
        const db = await this.initDB();
        return db.get(STORES.DIAGNOSIS_RESULTS, imageHash);
    }

    async getDiagnosisByWorkOrder(workOrderId) {
        const db = await this.initDB();
        const tx = db.transaction(STORES.DIAGNOSIS_RESULTS, 'readonly');
        const index = tx.store.index('workOrderId');
        const results = await index.getAll(workOrderId);
        return results[0];
    }

    async saveImage(hash, imageData) {
        const db = await this.initDB();
        await db.put(STORES.IMAGES, {
            hash,
            data: imageData,
            savedAt: new Date().toISOString()
        });
    }

    async getImage(hash) {
        const db = await this.initDB();
        const record = await db.get(STORES.IMAGES, hash);
        return record ? record.data : null;
    }

    async saveMessage(message) {
        const db = await this.initDB();
        const existing = await db.get(STORES.MESSAGES, message.id);
        
        if (existing) {
            await db.put(STORES.MESSAGES, { ...existing, ...message });
        } else {
            await db.add(STORES.MESSAGES, message);
        }
        
        return message;
    }

    async getMessages(workOrderId, limit = 50) {
        const db = await this.initDB();
        const tx = db.transaction(STORES.MESSAGES, 'readonly');
        const index = tx.store.index('workOrderId');
        const messages = await index.getAll(workOrderId);
        
        return messages
            .sort((a, b) => new Date(b.createdAt) - new Date(a.createdAt))
            .slice(0, limit)
            .reverse();
    }

    async addToSyncQueue(type, action, data) {
        const db = await this.initDB();
        await db.add(STORES.SYNC_QUEUE, {
            type,
            action,
            data,
            status: 'pending',
            createdAt: new Date().toISOString(),
            attempts: 0
        });
    }

    async processSyncQueue(apiClient) {
        if (!this.isOnline) {
            return { success: false, reason: 'offline' };
        }

        const db = await this.initDB();
        const syncItems = await db.getAll(STORES.SYNC_QUEUE);
        
        let processed = 0;
        let failed = 0;

        for (const item of syncItems) {
            if (item.status === 'processing' || item.attempts >= 3) {
                continue;
            }

            try {
                await db.put(STORES.SYNC_QUEUE, { ...item, status: 'processing' });
                
                let result;
                switch (item.type) {
                    case 'work_order':
                        if (item.action === 'create') {
                            result = await apiClient.post('/work-orders', item.data);
                            const localWo = await db.get(STORES.WORK_ORDERS, item.data.id);
                            if (localWo) {
                                localWo.isOfflineCreated = false;
                                localWo.offlineSyncStatus = 'synced';
                                await db.put(STORES.WORK_ORDERS, localWo);
                            }
                        }
                        break;
                    
                    case 'message':
                        if (item.action === 'send') {
                            result = await apiClient.post(`/work-orders/${item.data.workOrderId}/messages`, item.data);
                        }
                        break;
                }

                await db.delete(STORES.SYNC_QUEUE, item.id);
                processed++;
                
            } catch (error) {
                console.error(`Sync failed for item ${item.id}:`, error);
                item.attempts++;
                item.status = item.attempts >= 3 ? 'failed' : 'pending';
                item.lastError = error.message;
                await db.put(STORES.SYNC_QUEUE, item);
                failed++;
            }
        }

        return {
            success: true,
            processed,
            failed,
            remaining: syncItems.length - processed
        };
    }

    async setUserData(key, value) {
        const db = await this.initDB();
        await db.put(STORES.USER_DATA, { key, value });
    }

    async getUserData(key) {
        const db = await this.initDB();
        const record = await db.get(STORES.USER_DATA, key);
        return record ? record.value : null;
    }

    async clearAll() {
        const db = await this.initDB();
        await db.clear(STORES.WORK_ORDERS);
        await db.clear(STORES.MESSAGES);
        await db.clear(STORES.DIAGNOSIS_RESULTS);
        await db.clear(STORES.IMAGES);
        await db.clear(STORES.SYNC_QUEUE);
    }

    isNetworkOnline() {
        return this.isOnline;
    }
}

export const offlineStorage = new OfflineStorage();
