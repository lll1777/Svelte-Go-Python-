import { writable, derived, get } from 'svelte/store';
import { apiClient } from '$lib/api/client';
import { offlineStorage } from '$lib/utils/offlineStorage';
import { auth } from './auth';

const STATUS_ICONS = {
    pending: '⏳',
    diagnosing: '🔍',
    assigned: '👨‍🌾',
    consulting: '💬',
    prescribed: '📋',
    confirmed: '✅',
    follow_up: '🔄',
    closed: '🏁',
    cancelled: '❌'
};

const STATUS_LABELS = {
    pending: '待诊断',
    diagnosing: 'AI诊断中',
    assigned: '已分配专家',
    consulting: '问诊中',
    prescribed: '已开处方',
    confirmed: '用户确认',
    follow_up: '回访中',
    closed: '已关闭',
    cancelled: '已取消'
};

function createWorkOrderStore() {
    const { subscribe, set, update } = writable({
        workOrders: [],
        currentWorkOrder: null,
        diagnosisResults: {},
        isLoading: false,
        error: null,
        pagination: {
            page: 1,
            pageSize: 20,
            total: 0
        }
    });

    return {
        subscribe,

        async createWorkOrder(workOrderData) {
            update(state => ({ ...state, isLoading: true, error: null }));

            try {
                if (!navigator.onLine) {
                    const offlineWo = {
                        ...workOrderData,
                        id: `offline_${Date.now()}`,
                        status: 'pending',
                        isOfflineCreated: true,
                        offlineSyncStatus: 'pending',
                        createdAt: new Date().toISOString(),
                        updatedAt: new Date().toISOString()
                    };

                    await offlineStorage.saveWorkOrder(offlineWo);

                    update(state => ({
                        ...state,
                        isLoading: false,
                        workOrders: [offlineWo, ...state.workOrders]
                    }));

                    return { 
                        success: true, 
                        workOrder: offlineWo, 
                        isOffline: true 
                    };
                }

                const response = await apiClient.post('/work-orders', workOrderData);
                
                update(state => ({
                    ...state,
                    isLoading: false,
                    workOrders: [response, ...state.workOrders]
                }));

                return { success: true, workOrder: response };
            } catch (error) {
                update(state => ({
                    ...state,
                    isLoading: false,
                    error: error.message
                }));
                return { success: false, error: error.message };
            }
        },

        async uploadAndDiagnose(formData) {
            update(state => ({ ...state, isLoading: true, error: null }));

            try {
                if (!navigator.onLine) {
                    return { 
                        success: false, 
                        error: '图片诊断需要网络连接，请检查网络后重试',
                        isOffline: true
                    };
                }

                const response = await apiClient.upload('/work-orders/upload-diagnose', formData);
                
                const workOrder = response.work_order;
                const imageHashes = response.image_hashes || [];

                if (workOrder.diagnosis_result) {
                    const diagnosis = {
                        ...workOrder.diagnosis_result,
                        workOrderId: workOrder.id,
                        imageHashes
                    };
                    await offlineStorage.saveDiagnosisResult(diagnosis);
                }

                await offlineStorage.saveWorkOrder(workOrder);

                update(state => ({
                    ...state,
                    isLoading: false,
                    currentWorkOrder: workOrder,
                    workOrders: [workOrder, ...state.workOrders],
                    diagnosisResults: {
                        ...state.diagnosisResults,
                        [workOrder.id]: workOrder.diagnosis_result
                    }
                }));

                return { 
                    success: true, 
                    workOrder, 
                    imageHashes,
                    primaryImageHash: response.primary_image_hash
                };
            } catch (error) {
                update(state => ({
                    ...state,
                    isLoading: false,
                    error: error.message
                }));
                return { success: false, error: error.message };
            }
        },

        async fetchMyWorkOrders(status = null, page = 1, pageSize = 20) {
            update(state => ({ ...state, isLoading: true, error: null }));

            try {
                let workOrders;
                let total = 0;

                if (navigator.onLine) {
                    const params = new URLSearchParams();
                    if (status) params.append('status', status);
                    params.append('page', page.toString());
                    params.append('page_size', pageSize.toString());

                    const response = await apiClient.get(`/work-orders/my?${params.toString()}`);
                    workOrders = response.data || [];
                    total = response.total || 0;

                    for (const wo of workOrders) {
                        await offlineStorage.saveWorkOrder(wo);
                    }
                } else {
                    const authState = get(auth);
                    const farmerId = authState.user?.id;
                    workOrders = await offlineStorage.getWorkOrders(farmerId, status);
                    total = workOrders.length;
                }

                update(state => ({
                    ...state,
                    isLoading: false,
                    workOrders,
                    pagination: {
                        page,
                        pageSize,
                        total
                    }
                }));

                return { success: true, workOrders, total };
            } catch (error) {
                const authState = get(auth);
                const farmerId = authState.user?.id;
                const workOrders = await offlineStorage.getWorkOrders(farmerId, status);

                update(state => ({
                    ...state,
                    isLoading: false,
                    workOrders,
                    error: error.message
                }));

                return { success: false, workOrders, error: error.message };
            }
        },

        async fetchWorkOrder(id) {
            update(state => ({ ...state, isLoading: true, error: null }));

            try {
                let workOrder;

                if (navigator.onLine) {
                    const response = await apiClient.get(`/work-orders/${id}`);
                    workOrder = response.work_order;
                    await offlineStorage.saveWorkOrder(workOrder);
                } else {
                    workOrder = await offlineStorage.getWorkOrder(id);
                }

                if (!workOrder) {
                    throw new Error('Work order not found');
                }

                update(state => ({
                    ...state,
                    isLoading: false,
                    currentWorkOrder: workOrder
                }));

                return { success: true, workOrder };
            } catch (error) {
                update(state => ({
                    ...state,
                    isLoading: false,
                    error: error.message
                }));
                return { success: false, error: error.message };
            }
        },

        async updateStatus(workOrderId, newStatus, reason = '') {
            update(state => ({ ...state, isLoading: true, error: null }));

            try {
                if (!navigator.onLine) {
                    const workOrder = await offlineStorage.getWorkOrder(workOrderId);
                    if (workOrder) {
                        workOrder.status = newStatus;
                        workOrder.updatedAt = new Date().toISOString();
                        await offlineStorage.saveWorkOrder(workOrder);

                        update(state => {
                            const updated = state.workOrders.map(wo => 
                                wo.id === workOrderId ? { ...wo, status: newStatus } : wo
                            );
                            return {
                                ...state,
                                isLoading: false,
                                workOrders: updated,
                                currentWorkOrder: state.currentWorkOrder?.id === workOrderId 
                                    ? { ...state.currentWorkOrder, status: newStatus }
                                    : state.currentWorkOrder
                            };
                        });

                        return { 
                            success: true, 
                            isOffline: true,
                            message: '状态已保存，联网后将同步'
                        };
                    }
                }

                await apiClient.patch(`/work-orders/${workOrderId}/status`, {
                    new_status: newStatus,
                    reason
                });

                const { workOrder } = await this.fetchWorkOrder(workOrderId);

                return { success: true, workOrder };
            } catch (error) {
                update(state => ({
                    ...state,
                    isLoading: false,
                    error: error.message
                }));
                return { success: false, error: error.message };
            }
        },

        async submitFeedback(workOrderId, feedbackData) {
            update(state => ({ ...state, isLoading: true, error: null }));

            try {
                const response = await apiClient.post(`/work-orders/${workOrderId}/feedback`, feedbackData);

                await this.fetchWorkOrder(workOrderId);

                update(state => ({
                    ...state,
                    isLoading: false
                }));

                return { success: true, data: response };
            } catch (error) {
                update(state => ({
                    ...state,
                    isLoading: false,
                    error: error.message
                }));
                return { success: false, error: error.message };
            }
        },

        async checkImageAssociation(imageHash) {
            try {
                if (!navigator.onLine) {
                    const diagnosis = await offlineStorage.getDiagnosisByHash(imageHash);
                    if (diagnosis) {
                        return {
                            success: true,
                            is_associated: true,
                            work_order_id: diagnosis.workOrderId
                        };
                    }
                    return {
                        success: true,
                        is_associated: false,
                        message: '离线模式下未找到关联记录'
                    };
                }

                const response = await apiClient.get(`/work-orders/check-image-association?image_hash=${encodeURIComponent(imageHash)}`);
                return response;
            } catch (error) {
                return { success: false, error: error.message };
            }
        },

        setCurrentWorkOrder(workOrder) {
            update(state => ({
                ...state,
                currentWorkOrder: workOrder
            }));
        },

        clearError() {
            update(state => ({ ...state, error: null }));
        }
    };
}

export const workOrders = createWorkOrderStore();

export const statusIcons = STATUS_ICONS;
export const statusLabels = STATUS_LABELS;

export const pendingWorkOrders = derived(workOrders, $store => 
    $store.workOrders.filter(wo => ['pending', 'diagnosing', 'assigned', 'consulting'].includes(wo.status))
);

export const completedWorkOrders = derived(workOrders, $store => 
    $store.workOrders.filter(wo => ['prescribed', 'confirmed', 'follow_up', 'closed'].includes(wo.status))
);
