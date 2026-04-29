<script>
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { auth, isAuthenticated, userRole } from '$stores/auth';
	import { workOrders, statusIcons, statusLabels } from '$stores/workOrders';

	let loading = $state(true);
	let activeTab = $state('pending');
	let selectedWorkOrder = $state(null);
	let showPrescriptionModal = $state(false);

	let prescriptionData = $state({
		diagnosis: '',
		treatmentPlan: '',
		medications: '',
		dosage: '',
		applicationMethod: '',
		preventionTips: '',
		notes: ''
	});

	let isSubmittingPrescription = $state(false);
	let prescriptionError = $state('');

	const tabs = [
		{ id: 'pending', label: '待诊断', icon: '⏳' },
		{ id: 'assigned', label: '我的工单', icon: '📋' },
		{ id: 'completed', label: '已完成', icon: '✅' }
	];

	$: filteredOrders = $workOrders.workOrders.filter(wo => {
		if (activeTab === 'pending') {
			return ['pending', 'diagnosing'].includes(wo.status);
		} else if (activeTab === 'assigned') {
			return ['assigned', 'consulting', 'prescribed'].includes(wo.status);
		} else if (activeTab === 'completed') {
			return ['confirmed', 'closed'].includes(wo.status);
		}
		return false;
	});

	onMount(async () => {
		if (!$isAuthenticated) {
			goto('/login');
			return;
		}

		if ($userRole !== 'expert' && $userRole !== 'admin') {
			goto('/');
			return;
		}

		await workOrders.fetchMyWorkOrders();
		loading = false;
	});

	async function handleTabChange(tabId) {
		activeTab = tabId;
		selectedWorkOrder = null;
	}

	async function selectWorkOrder(order) {
		selectedWorkOrder = order;
		await workOrders.fetchWorkOrder(order.id);
		selectedWorkOrder = $workOrders.currentWorkOrder;
	}

	async function openPrescriptionModal() {
		if (!selectedWorkOrder) return;
		
		prescriptionData = {
			diagnosis: selectedWorkOrder.diagnosisResult?.diseaseName || '',
			treatmentPlan: '',
			medications: '',
			dosage: '',
			applicationMethod: '',
			preventionTips: '',
			notes: ''
		};
		
		prescriptionError = '';
		showPrescriptionModal = true;
	}

	async function submitPrescription() {
		if (!prescriptionData.diagnosis.trim()) {
			prescriptionError = '请输入诊断结果';
			return;
		}

		isSubmittingPrescription = true;
		prescriptionError = '';

		try {
			const result = await workOrders.createPrescription(selectedWorkOrder.id, prescriptionData);
			
			if (result.success) {
				showPrescriptionModal = false;
				await workOrders.fetchWorkOrder(selectedWorkOrder.id);
				selectedWorkOrder = $workOrders.currentWorkOrder;
			} else {
				throw new Error(result.error);
			}
		} catch (err) {
			prescriptionError = err.message || '提交处方失败';
		} finally {
			isSubmittingPrescription = false;
		}
	}

	function getStatusColor(status) {
		const colorMap = {
			pending: 'bg-yellow-100 text-yellow-800',
			diagnosing: 'bg-blue-100 text-blue-800',
			assigned: 'bg-purple-100 text-purple-800',
			consulting: 'bg-cyan-100 text-cyan-800',
			prescribed: 'bg-green-100 text-green-800',
			confirmed: 'bg-emerald-100 text-emerald-800',
			closed: 'bg-gray-100 text-gray-800'
		};
		return colorMap[status] || 'bg-gray-100 text-gray-800';
	}
</script>

<div class="workbench-container">
	<header class="page-header">
		<h1>👨‍🔬 专家工作台</h1>
		<p>管理诊断工单和问诊服务</p>
	</header>

	{#if loading}
		<div class="loading-state">
			<div class="spinner-large"></div>
			<p>加载中...</p>
		</div>
	{:else}
		<div class="workbench-layout">
			<section class="workorder-list-panel">
				<div class="tabs">
					{#each tabs as tab}
						<button
							class="tab-btn {activeTab === tab.id ? 'active' : ''}"
							on:click={() => handleTabChange(tab.id)}
						>
							<span class="tab-icon">{tab.icon}</span>
							<span>{tab.label}</span>
							<span class="tab-count">
								{filteredOrders.filter(wo => {
									if (tab.id === 'pending') return ['pending', 'diagnosing'].includes(wo.status);
									if (tab.id === 'assigned') return ['assigned', 'consulting', 'prescribed'].includes(wo.status);
									if (tab.id === 'completed') return ['confirmed', 'closed'].includes(wo.status);
									return false;
								}).length}
							</span>
						</button>
					{/each}
				</div>

				<div class="workorder-list">
					{#if filteredOrders.length > 0}
						{#each filteredOrders as order}
							<div
								class="workorder-item {selectedWorkOrder?.id === order.id ? 'selected' : ''}"
								on:click={() => selectWorkOrder(order)}
							>
								<div class="item-header">
									<h4>{order.title}</h4>
									<span class="status-badge {getStatusColor(order.status)}">
										{statusLabels[order.status]}
									</span>
								</div>
								<p class="item-desc">{order.description}</p>
								<div class="item-meta">
									<span class="crop-type">🌱 {order.cropType || '未知'}</span>
									<span class="time">
										{new Date(order.createdAt).toLocaleDateString('zh-CN', {
											month: 'short',
											day: 'numeric',
											hour: '2-digit',
											minute: '2-digit'
										})}
									</span>
								</div>
								{#if order.diagnosisResult}
									<div class="ai-diagnosis-preview">
										<span class="ai-badge">AI</span>
										{order.diagnosisResult.diseaseName}
									</div>
								{/if}
							</div>
						{/each}
					{:else}
						<div class="empty-list">
							<span class="empty-icon">📭</span>
							<p>暂无{activeTab === 'pending' ? '待诊断' : activeTab === 'assigned' ? '进行中' : '已完成'}工单</p>
						</div>
					{/if}
				</div>
			</section>

			<section class="workorder-detail-panel">
				{#if selectedWorkOrder}
					<div class="detail-content">
						<div class="detail-header">
							<h2>{selectedWorkOrder.title}</h2>
							<span class="status-badge {getStatusColor(selectedWorkOrder.status)}">
								{statusIcons[selectedWorkOrder.status]} {statusLabels[selectedWorkOrder.status]}
							</span>
						</div>

						<div class="farmer-info">
							<h4>农户信息</h4>
							<div class="info-row">
								<span class="info-label">姓名：</span>
								<span class="info-value">{selectedWorkOrder.farmer?.fullName || '未知'}</span>
							</div>
							{#if selectedWorkOrder.location}
								<div class="info-row">
									<span class="info-label">📍 位置：</span>
									<span class="info-value">{selectedWorkOrder.location}</span>
								</div>
							{/if}
							<div class="info-row">
								<span class="info-label">🌱 作物：</span>
								<span class="info-value">{selectedWorkOrder.cropType || '未知'}</span>
							</div>
						</div>

						<div class="problem-description">
							<h4>问题描述</h4>
							<p>{selectedWorkOrder.description}</p>
						</div>

						{#if selectedWorkOrder.diagnosisResult}
							<div class="ai-diagnosis-section">
								<h4>🤖 AI诊断结果</h4>
								<div class="diagnosis-content">
									<div class="diagnosis-name">
										<span class="name">{selectedWorkOrder.diagnosisResult.diseaseName}</span>
										<span class="confidence">
											{Math.round(selectedWorkOrder.diagnosisResult.confidence * 100)}%
										</span>
									</div>
									{#if selectedWorkOrder.diagnosisResult.symptoms}
										<div class="info-item">
											<span class="label">症状：</span>
											<span class="value">{selectedWorkOrder.diagnosisResult.symptoms}</span>
										</div>
									{/if}
									{#if selectedWorkOrder.diagnosisResult.recommendedActions}
										<div class="info-item">
											<span class="label">推荐措施：</span>
											<span class="value">{selectedWorkOrder.diagnosisResult.recommendedActions}</span>
										</div>
									{/if}
								</div>
							</div>
						{/if}

						{#if selectedWorkOrder.prescription}
							<div class="prescription-section">
								<h4>📋 已开具处方</h4>
								<div class="prescription-content">
									{#if selectedWorkOrder.prescription.diagnosis}
										<div class="info-item">
											<span class="label">诊断：</span>
											<span class="value">{selectedWorkOrder.prescription.diagnosis}</span>
										</div>
									{/if}
									{#if selectedWorkOrder.prescription.treatmentPlan}
										<div class="info-item">
											<span class="label">治疗方案：</span>
											<span class="value">{selectedWorkOrder.prescription.treatmentPlan}</span>
										</div>
									{/if}
									{#if selectedWorkOrder.prescription.medications}
										<div class="info-item">
											<span class="label">推荐用药：</span>
											<span class="value">{selectedWorkOrder.prescription.medications}</span>
										</div>
									{/if}
								</div>
							</div>
						{/if}

						<div class="action-buttons">
							{#if ['assigned', 'consulting'].includes(selectedWorkOrder.status)}
								{#if !selectedWorkOrder.prescription}
									<button
										class="btn btn-primary"
										on:click={openPrescriptionModal}
									>
										📝 开具处方
									</button>
								{/if}
								<a
									href="/work-orders/{selectedWorkOrder.id}"
									class="btn btn-secondary"
								>
									💬 在线问诊
								</a>
							{/if}

							{#if selectedWorkOrder.feedback}
								<a
									href="/work-orders/{selectedWorkOrder.id}"
									class="btn btn-secondary"
								>
									⭐ 查看反馈
								</a>
							{/if}
						</div>
					</div>
				{:else}
					<div class="empty-detail">
						<span class="empty-icon">👆</span>
						<p>请从左侧选择一个工单查看详情</p>
					</div>
				{/if}
			</section>
		</div>
	{/if}

	{#if showPrescriptionModal}
		<div class="modal-overlay" on:click={() => showPrescriptionModal = false}>
			<div class="modal" on:click|stopPropagation>
				<div class="modal-header">
					<h3>📝 开具处方</h3>
					<button class="close-btn" on:click={() => showPrescriptionModal = false}>✕</button>
				</div>

				{#if prescriptionError}
					<div class="alert alert-error">{prescriptionError}</div>
				{/if}

				<div class="modal-body">
					<div class="form-group">
						<label for="diagnosis">诊断结果 *</label>
						<textarea
							id="diagnosis"
							bind:value={prescriptionData.diagnosis}
							placeholder="输入专业诊断结果"
							rows="2"
							required
						></textarea>
					</div>

					<div class="form-group">
						<label for="treatmentPlan">治疗方案</label>
						<textarea
							id="treatmentPlan"
							bind:value={prescriptionData.treatmentPlan}
							placeholder="详细描述治疗方案"
							rows="3"
						></textarea>
					</div>

					<div class="form-group">
						<label for="medications">推荐用药</label>
						<textarea
							id="medications"
							bind:value={prescriptionData.medications}
							placeholder="列出推荐使用的农药"
							rows="2"
						></textarea>
					</div>

					<div class="form-group">
						<label for="dosage">使用剂量</label>
						<textarea
							id="dosage"
							bind:value={prescriptionData.dosage}
							placeholder="详细的使用剂量和方法"
							rows="2"
						></textarea>
					</div>

					<div class="form-group">
						<label for="applicationMethod">施用方法</label>
						<textarea
							id="applicationMethod"
							bind:value={prescriptionData.applicationMethod}
							placeholder="具体的施用方法说明"
							rows="2"
						></textarea>
					</div>

					<div class="form-group">
						<label for="preventionTips">预防建议</label>
						<textarea
							id="preventionTips"
							bind:value={prescriptionData.preventionTips}
							placeholder="日常预防和管理建议"
							rows="2"
						></textarea>
					</div>

					<div class="form-group">
						<label for="notes">备注</label>
						<textarea
							id="notes"
							bind:value={prescriptionData.notes}
							placeholder="其他需要说明的事项"
							rows="2"
						></textarea>
					</div>
				</div>

				<div class="modal-footer">
					<button
						type="button"
						class="btn btn-secondary"
						on:click={() => showPrescriptionModal = false}
					>
						取消
					</button>
					<button
						type="button"
						class="btn btn-primary"
						disabled={isSubmittingPrescription}
						on:click={submitPrescription}
					>
						{#if isSubmittingPrescription}
							<span class="spinner"></span>
							<span>提交中...</span>
						{:else}
							提交处方
						{/if}
					</button>
				</div>
			</div>
		</div>
	{/if}
</div>

<style>
	.workbench-container {
		height: calc(100vh - 120px);
		display: flex;
		flex-direction: column;
	}

	.page-header {
		margin-bottom: 20px;
		flex-shrink: 0;
	}

	.page-header h1 {
		font-size: 24px;
		margin: 0 0 8px 0;
	}

	.page-header p {
		margin: 0;
		color: #666;
		font-size: 14px;
	}

	.loading-state {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		flex: 1;
	}

	.spinner-large {
		width: 40px;
		height: 40px;
		border: 3px solid #e5e5e5;
		border-top-color: #2d5a27;
		border-radius: 50%;
		animation: spin 0.8s linear infinite;
		margin-bottom: 16px;
	}

	@keyframes spin {
		to { transform: rotate(360deg); }
	}

	.workbench-layout {
		display: flex;
		gap: 20px;
		flex: 1;
		min-height: 0;
	}

	.workorder-list-panel {
		width: 380px;
		flex-shrink: 0;
		background: white;
		border-radius: 12px;
		display: flex;
		flex-direction: column;
		box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
	}

	.tabs {
		display: flex;
		padding: 12px;
		gap: 8px;
		border-bottom: 1px solid #e5e5e5;
	}

	.tab-btn {
		flex: 1;
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 6px;
		padding: 10px 12px;
		background: #f3f4f6;
		border: none;
		border-radius: 8px;
		font-size: 13px;
		cursor: pointer;
		transition: all 0.2s;
	}

	.tab-btn:hover {
		background: #e5e7eb;
	}

	.tab-btn.active {
		background: #2d5a27;
		color: white;
	}

	.tab-icon {
		font-size: 14px;
	}

	.tab-count {
		background: rgba(0, 0, 0, 0.1);
		padding: 2px 8px;
		border-radius: 10px;
		font-size: 11px;
	}

	.tab-btn.active .tab-count {
		background: rgba(255, 255, 255, 0.2);
	}

	.workorder-list {
		flex: 1;
		overflow-y: auto;
		padding: 12px;
	}

	.workorder-item {
		padding: 16px;
		border-radius: 8px;
		cursor: pointer;
		transition: all 0.2s;
		border: 1px solid transparent;
		margin-bottom: 8px;
	}

	.workorder-item:hover {
		background: #f9fafb;
	}

	.workorder-item.selected {
		background: #f0f7ef;
		border-color: #2d5a27;
	}

	.item-header {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		gap: 12px;
		margin-bottom: 8px;
	}

	.item-header h4 {
		margin: 0;
		font-size: 14px;
		color: #1a1a1a;
		flex: 1;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.status-badge {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		padding: 3px 10px;
		border-radius: 12px;
		font-size: 11px;
		font-weight: 500;
		white-space: nowrap;
	}

	.item-desc {
		margin: 0 0 10px 0;
		font-size: 13px;
		color: #666;
		display: -webkit-box;
		-webkit-line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
	}

	.item-meta {
		display: flex;
		justify-content: space-between;
		font-size: 12px;
		color: #888;
		margin-bottom: 8px;
	}

	.ai-diagnosis-preview {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 6px 10px;
		background: #f0f7ef;
		border-radius: 6px;
		font-size: 12px;
		color: #2d5a27;
	}

	.ai-badge {
		padding: 2px 6px;
		background: #2d5a27;
		color: white;
		border-radius: 3px;
		font-size: 10px;
		font-weight: 600;
	}

	.empty-list {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		padding: 40px 20px;
		color: #9ca3af;
		text-align: center;
	}

	.empty-icon {
		font-size: 36px;
		margin-bottom: 12px;
	}

	.workorder-detail-panel {
		flex: 1;
		background: white;
		border-radius: 12px;
		overflow-y: auto;
		box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
	}

	.empty-detail {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		height: 100%;
		color: #9ca3af;
	}

	.empty-detail .empty-icon {
		font-size: 48px;
		margin-bottom: 16px;
	}

	.detail-content {
		padding: 24px;
	}

	.detail-header {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		gap: 16px;
		margin-bottom: 24px;
		padding-bottom: 20px;
		border-bottom: 1px solid #e5e5e5;
	}

	.detail-header h2 {
		margin: 0;
		font-size: 18px;
		color: #1a1a1a;
		flex: 1;
	}

	.farmer-info,
	.problem-description,
	.ai-diagnosis-section,
	.prescription-section {
		margin-bottom: 24px;
	}

	.farmer-info h4,
	.problem-description h4,
	.ai-diagnosis-section h4,
	.prescription-section h4 {
		margin: 0 0 12px 0;
		font-size: 14px;
		color: #555;
		font-weight: 600;
	}

	.info-row {
		display: flex;
		margin-bottom: 8px;
		font-size: 14px;
	}

	.info-label {
		color: #888;
		min-width: 80px;
	}

	.info-value {
		color: #333;
		flex: 1;
	}

	.problem-description p {
		margin: 0;
		line-height: 1.6;
		color: #333;
	}

	.diagnosis-content,
	.prescription-content {
		padding: 16px;
		background: #f9fafb;
		border-radius: 8px;
	}

	.diagnosis-name {
		display: flex;
		align-items: baseline;
		gap: 12px;
		margin-bottom: 12px;
	}

	.diagnosis-name .name {
		font-size: 18px;
		font-weight: 600;
		color: #2d5a27;
	}

	.diagnosis-name .confidence {
		font-size: 13px;
		color: #666;
		background: white;
		padding: 2px 10px;
		border-radius: 12px;
	}

	.info-item {
		display: flex;
		margin-bottom: 8px;
		font-size: 14px;
	}

	.info-item:last-child {
		margin-bottom: 0;
	}

	.info-item .label {
		color: #888;
		min-width: 80px;
		flex-shrink: 0;
	}

	.info-item .value {
		color: #333;
		line-height: 1.5;
	}

	.action-buttons {
		display: flex;
		gap: 12px;
		padding-top: 20px;
		border-top: 1px solid #e5e5e5;
	}

	.btn {
		padding: 12px 24px;
		border-radius: 8px;
		font-size: 14px;
		font-weight: 500;
		cursor: pointer;
		transition: all 0.2s;
		border: none;
		display: inline-flex;
		align-items: center;
		gap: 8px;
		text-decoration: none;
	}

	.btn:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.btn-primary {
		background-color: #2d5a27;
		color: white;
	}

	.btn-primary:hover:not(:disabled) {
		background-color: #1f431b;
	}

	.btn-secondary {
		background-color: #f3f4f6;
		color: #374151;
	}

	.btn-secondary:hover:not(:disabled) {
		background-color: #e5e7eb;
	}

	.spinner {
		width: 16px;
		height: 16px;
		border: 2px solid rgba(255, 255, 255, 0.3);
		border-top-color: white;
		border-radius: 50%;
		animation: spin 0.8s linear infinite;
	}

	.modal-overlay {
		position: fixed;
		top: 0;
		left: 0;
		right: 0;
		bottom: 0;
		background: rgba(0, 0, 0, 0.5);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 1000;
		padding: 20px;
	}

	.modal {
		background: white;
		border-radius: 12px;
		width: 100%;
		max-width: 600px;
		max-height: 90vh;
		display: flex;
		flex-direction: column;
	}

	.modal-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 20px 24px;
		border-bottom: 1px solid #e5e5e5;
		flex-shrink: 0;
	}

	.modal-header h3 {
		margin: 0;
		font-size: 18px;
	}

	.close-btn {
		background: none;
		border: none;
		font-size: 20px;
		cursor: pointer;
		color: #666;
		padding: 4px;
	}

	.close-btn:hover {
		color: #333;
	}

	.alert {
		padding: 12px 16px;
		border-radius: 8px;
		margin: 0 24px 16px;
		font-size: 14px;
	}

	.alert-error {
		background-color: #fef2f2;
		color: #dc2626;
		border: 1px solid #fecaca;
	}

	.modal-body {
		flex: 1;
		overflow-y: auto;
		padding: 24px;
	}

	.form-group {
		margin-bottom: 16px;
	}

	.form-group label {
		display: block;
		margin-bottom: 8px;
		font-weight: 500;
		color: #374151;
		font-size: 14px;
	}

	.form-group textarea {
		width: 100%;
		padding: 12px 16px;
		border: 1px solid #d1d5db;
		border-radius: 8px;
		font-size: 14px;
		box-sizing: border-box;
		resize: vertical;
		min-height: 60px;
	}

	.form-group textarea:focus {
		outline: none;
		border-color: #2d5a27;
	}

	.modal-footer {
		display: flex;
		justify-content: flex-end;
		gap: 12px;
		padding: 16px 24px;
		border-top: 1px solid #e5e5e5;
		flex-shrink: 0;
	}

	@media (max-width: 1024px) {
		.workbench-layout {
			flex-direction: column;
		}

		.workorder-list-panel {
			width: 100%;
			height: 40%;
			min-height: 200px;
		}

		.workorder-detail-panel {
			height: 60%;
			min-height: 300px;
		}
	}

	@media (max-width: 640px) {
		.tabs {
			flex-wrap: wrap;
		}

		.tab-btn {
			min-width: calc(33.33% - 6px);
		}

		.action-buttons {
			flex-direction: column;
		}

		.btn {
			width: 100%;
			justify-content: center;
		}

		.modal-footer {
			flex-direction: column;
		}

		.modal-footer .btn {
			width: 100%;
			justify-content: center;
		}
	}
</style>
