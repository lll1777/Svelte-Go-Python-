<script>
	import { onMount, onDestroy } from 'svelte';
	import { page, goto } from '$app/stores';
	import { auth, isAuthenticated, userRole } from '$stores/auth';
	import { workOrders, statusIcons, statusLabels } from '$stores/workOrders';
	import { wsService } from '$lib/services/websocket';
	import { offlineStorage } from '$lib/utils/offlineStorage';

	let workOrderId = $derived($page.params.id);
	let workOrder = $state(null);
	let loading = $state(true);
	let error = $state('');

	let messages = $state([]);
	let newMessage = $state('');
	let isSending = $state(false);

	let showFeedbackForm = $state(false);
	let feedbackData = $state({
		rating: 5,
		effectiveness: '',
		comments: '',
		improvements: '',
		isSolved: true
	});

	let isSubmittingFeedback = $state(false);

	$: isFarmer = $userRole === 'farmer';
	$: isExpert = $userRole === 'expert';

	let messagesUnsubscribe = null;

	onMount(async () => {
		if (!$isAuthenticated) {
			goto('/login');
			return;
		}

		await loadWorkOrder();

		messagesUnsubscribe = wsService.messages.subscribe(value => {
			messages = value;
		});

		wsService.joinWorkOrder(workOrderId);
	});

	onDestroy(() => {
		if (messagesUnsubscribe) {
			messagesUnsubscribe();
		}
		wsService.leaveWorkOrder(workOrderId);
	});

	async function loadWorkOrder() {
		loading = true;
		error = '';

		try {
			const result = await workOrders.fetchWorkOrder(workOrderId);
			if (result.success) {
				workOrder = result.workOrder;
			} else {
				throw new Error(result.error);
			}
		} catch (err) {
			error = err.message || '加载工单失败';
		} finally {
			loading = false;
		}
	}

	async function sendMessage() {
		if (!newMessage.trim() || isSending) return;

		isSending = true;
		const messageContent = newMessage.trim();
		newMessage = '';

		try {
			await wsService.sendMessage(workOrderId, messageContent);
		} catch (err) {
			console.error('Failed to send message:', err);
		} finally {
			isSending = false;
		}
	}

	async function submitFeedback() {
		if (feedbackData.rating < 1 || feedbackData.rating > 5) {
			error = '请选择1-5星评分';
			return;
		}

		isSubmittingFeedback = true;
		error = '';

		try {
			const result = await workOrders.submitFeedback(workOrderId, feedbackData);
			if (result.success) {
				showFeedbackForm = false;
				await loadWorkOrder();
			} else {
				throw new Error(result.error);
			}
		} catch (err) {
			error = err.message || '提交反馈失败';
		} finally {
			isSubmittingFeedback = false;
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
			follow_up: 'bg-orange-100 text-orange-800',
			closed: 'bg-gray-100 text-gray-800'
		};
		return colorMap[status] || 'bg-gray-100 text-gray-800';
	}

	function formatTime(dateStr) {
		if (!dateStr) return '';
		const date = new Date(dateStr);
		return date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' });
	}

	function isCurrentUser(senderId) {
		return senderId === $auth.user?.id;
	}

	const ratingStars = [1, 2, 3, 4, 5];
</script>

<div class="work-order-detail-container">
	<header class="page-header">
		<button class="back-btn" on:click={() => goto('/work-orders')}>
			← 返回列表
		</button>
	</header>

	{#if loading}
		<div class="loading-state">
			<div class="spinner-large"></div>
			<p>加载中...</p>
		</div>
	{:else if error}
		<div class="error-state">
			<span class="error-icon">❌</span>
			<h3>加载失败</h3>
			<p>{error}</p>
			<button class="btn btn-primary" on:click={loadWorkOrder}>重试</button>
		</div>
	{:else if workOrder}
		<div class="detail-content">
			<section class="order-info-card">
				<div class="order-header">
					<div class="order-title">
						<h2>{workOrder.title}</h2>
						<span class="status-badge {getStatusColor(workOrder.status)}">
							{statusIcons[workOrder.status]} {statusLabels[workOrder.status]}
						</span>
					</div>
					<div class="order-meta">
						<span class="meta-item">
							<span class="meta-icon">🌱</span>
							<span>{workOrder.cropType || '未知作物'}</span>
						</span>
						{#if workOrder.location}
							<span class="meta-item">
								<span class="meta-icon">📍</span>
								<span>{workOrder.location}</span>
							</span>
						{/if}
						<span class="meta-item">
							<span class="meta-icon">📅</span>
							<span>
								{new Date(workOrder.createdAt).toLocaleDateString('zh-CN', {
									year: 'numeric',
									month: 'long',
									day: 'numeric',
									hour: '2-digit',
									minute: '2-digit'
								})}
							</span>
						</span>
					</div>
				</div>

				<div class="order-description">
					<h4>问题描述</h4>
					<p>{workOrder.description}</p>
				</div>
			</section>

			{#if workOrder.diagnosisResult}
				<section class="diagnosis-card">
					<div class="section-header">
						<h3>🤖 AI诊断结果</h3>
						<span class="confidence">
							置信度：{Math.round(workOrder.diagnosisResult.confidence * 100)}%
						</span>
					</div>

					<div class="diagnosis-summary">
						<div class="disease-name">
							<span class="disease-label">诊断：</span>
							<span class="disease-value">{workOrder.diagnosisResult.diseaseName}</span>
							<span class="disease-type">{workOrder.diagnosisResult.diseaseType}</span>
						</div>
						{#if workOrder.diagnosisResult.severity}
							<span class="severity-badge">
								{workOrder.diagnosisResult.severity}
							</span>
						{/if}
					</div>

					{#if workOrder.diagnosisResult.symptoms}
						<div class="info-section">
							<h4>症状</h4>
							<p>{workOrder.diagnosisResult.symptoms}</p>
						</div>
					{/if}

					{#if workOrder.diagnosisResult.causes}
						<div class="info-section">
							<h4>发病原因</h4>
							<p>{workOrder.diagnosisResult.causes}</p>
						</div>
					{/if}

					{#if workOrder.diagnosisResult.recommendedActions}
						<div class="info-section">
							<h4>推荐措施</h4>
							<p>{workOrder.diagnosisResult.recommendedActions}</p>
						</div>
					{/if}
				</section>
			{/if}

			{#if workOrder.prescription}
				<section class="prescription-card">
					<div class="section-header">
						<h3>📋 专家处方</h3>
						{#if workOrder.expert}
							<span class="expert-info">
								👨‍🔬 {workOrder.expert.fullName}
							</span>
						{/if}
					</div>

					{#if workOrder.prescription.diagnosis}
						<div class="info-section">
							<h4>专家诊断</h4>
							<p>{workOrder.prescription.diagnosis}</p>
						</div>
					{/if}

					{#if workOrder.prescription.treatmentPlan}
						<div class="info-section">
							<h4>治疗方案</h4>
							<p>{workOrder.prescription.treatmentPlan}</p>
						</div>
					{/if}

					{#if workOrder.prescription.medications}
						<div class="info-section">
							<h4>推荐用药</h4>
							<p>{workOrder.prescription.medications}</p>
						</div>
					{/if}

					{#if workOrder.prescription.dosage}
						<div class="info-section">
							<h4>使用剂量</h4>
							<p>{workOrder.prescription.dosage}</p>
						</div>
					{/if}

					{#if workOrder.prescription.applicationMethod}
						<div class="info-section">
							<h4>施用方法</h4>
							<p>{workOrder.prescription.applicationMethod}</p>
						</div>
					{/if}

					{#if workOrder.prescription.preventionTips}
						<div class="info-section">
							<h4>预防建议</h4>
							<p>{workOrder.prescription.preventionTips}</p>
						</div>
					{/if}

					{#if workOrder.prescription.followUpDate}
						<div class="info-section">
							<h4>回访日期</h4>
							<p>{new Date(workOrder.prescription.followUpDate).toLocaleDateString('zh-CN')}</p>
						</div>
					{/if}

					{#if workOrder.prescription.notes}
						<div class="info-section">
							<h4>备注</h4>
							<p>{workOrder.prescription.notes}</p>
						</div>
					{/if}
				</section>
			{/if}

			{#if workOrder.feedback}
				<section class="feedback-card">
					<div class="section-header">
						<h3>⭐ 防治效果反馈</h3>
					</div>

					<div class="rating-display">
						{#each ratingStars as star}
							<span class="star {star <= workOrder.feedback.rating ? 'filled' : ''}">★</span>
						{/each}
						<span class="rating-text">{workOrder.feedback.rating} 分</span>
					</div>

					{#if workOrder.feedback.effectiveness}
						<div class="info-section">
							<h4>防治效果</h4>
							<p>{workOrder.feedback.effectiveness}</p>
						</div>
					{/if}

					{#if workOrder.feedback.comments}
						<div class="info-section">
							<h4>用户评价</h4>
							<p>{workOrder.feedback.comments}</p>
						</div>
					{/if}

					{#if workOrder.feedback.improvements}
						<div class="info-section">
							<h4>改进建议</h4>
							<p>{workOrder.feedback.improvements}</p>
						</div>
					{/if}

					<div class="solved-status">
						问题是否解决：
						<span class={workOrder.feedback.isSolved ? 'solved' : 'not-solved'}>
							{workOrder.feedback.isSolved ? '✅ 已解决' : '❌ 未解决'}
						</span>
					</div>
				</section>
			{/if}

			{#if isFarmer && ['prescribed', 'confirmed'].includes(workOrder.status) && !workOrder.feedback}
				<div class="feedback-actions">
					{#if showFeedbackForm}
						<div class="feedback-form">
							<h4>提交防治效果反馈</h4>
							
							<div class="form-group">
								<label>整体评分</label>
								<div class="rating-input">
									{#each ratingStars as star}
										<button
											type="button"
											class="star-btn {star <= feedbackData.rating ? 'active' : ''}"
											on:click={() => feedbackData.rating = star}
										>★</button>
									{/each}
									<span class="rating-value">{feedbackData.rating} 分</span>
								</div>
							</div>

							<div class="form-group">
								<label for="effectiveness">防治效果</label>
								<textarea
									id="effectiveness"
									bind:value={feedbackData.effectiveness}
									placeholder="描述防治效果如何..."
									rows="3"
								></textarea>
							</div>

							<div class="form-group">
								<label for="comments">评价</label>
								<textarea
									id="comments"
									bind:value={feedbackData.comments}
									placeholder="您对这次服务的评价..."
									rows="3"
								></textarea>
							</div>

							<div class="form-group">
								<label for="improvements">改进建议</label>
								<textarea
									id="improvements"
									bind:value={feedbackData.improvements}
									placeholder="您有什么建议帮助我们改进..."
									rows="3"
								></textarea>
							</div>

							<div class="form-group checkbox-group">
								<label>
									<input
										type="checkbox"
										bind:checked={feedbackData.isSolved}
									/>
									问题已解决
								</label>
							</div>

							<div class="form-actions">
								<button
									type="button"
									class="btn btn-secondary"
									on:click={() => showFeedbackForm = false}
								>
									取消
								</button>
								<button
									type="button"
									class="btn btn-primary"
									disabled={isSubmittingFeedback}
									on:click={submitFeedback}
								>
									{#if isSubmittingFeedback}
										<span class="spinner"></span>
										<span>提交中...</span>
									{:else}
										提交反馈
									{/if}
								</button>
							</div>
						</div>
					{:else}
						<button class="btn btn-primary btn-full" on:click={() => showFeedbackForm = true}>
							⭐ 提交防治效果反馈
						</button>
					{/if}
				</div>
			{/if}

			{#if ['assigned', 'consulting', 'prescribed', 'confirmed'].includes(workOrder.status)}
				<section class="chat-section">
					<div class="section-header">
						<h3>💬 在线问诊</h3>
						{#if $wsService.isConnected}
							<span class="connection-status online">● 已连接</span>
						{:else}
							<span class="connection-status offline">● 未连接</span>
						{/if}
					</div>

					<div class="chat-container">
						<div class="messages-container">
							{#if messages.length === 0}
								<div class="empty-messages">
									<span class="empty-icon">💬</span>
									<p>还没有消息，开始与专家交流吧</p>
								</div>
							{:else}
								{#each messages as msg}
									<div class="message-bubble {isCurrentUser(msg.sender_id) ? 'sent' : 'received'}">
										<div class="message-content">
											{msg.message_type === 'image' ? (
												<img src={msg.image_url} alt="图片" class="message-image" />
											) : (
												<p>{msg.content}</p>
											)}
											<span class="message-time">{formatTime(msg.created_at)}</span>
										</div>
									</div>
								{/each}
							{/if}
						</div>

						<div class="message-input-area">
							<input
								type="text"
								bind:value={newMessage}
								placeholder="输入消息..."
								on:keydown={(e) => e.key === 'Enter' && !e.shiftKey && sendMessage()}
								disabled={isSending}
							/>
							<button
								type="button"
								class="send-btn"
								disabled={!newMessage.trim() || isSending}
								on:click={sendMessage}
							>
								发送
							</button>
						</div>
					</div>
				</section>
			{/if}
		</div>
	{/if}
</div>

<style>
	.work-order-detail-container {
		max-width: 800px;
		margin: 0 auto;
	}

	.page-header {
		margin-bottom: 20px;
	}

	.back-btn {
		background: none;
		border: none;
		color: #2d5a27;
		font-size: 14px;
		cursor: pointer;
		padding: 8px 0;
	}

	.back-btn:hover {
		text-decoration: underline;
	}

	.loading-state,
	.error-state {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		padding: 80px 20px;
		text-align: center;
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

	.error-icon {
		font-size: 48px;
		margin-bottom: 16px;
	}

	.error-state h3 {
		margin: 0 0 8px 0;
		color: #dc2626;
	}

	.error-state p {
		margin: 0 0 20px 0;
		color: #666;
	}

	.detail-content {
		display: flex;
		flex-direction: column;
		gap: 20px;
	}

	.order-info-card,
	.diagnosis-card,
	.prescription-card,
	.feedback-card {
		background: white;
		border-radius: 12px;
		padding: 24px;
		box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
	}

	.order-header {
		margin-bottom: 16px;
	}

	.order-title {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		gap: 16px;
		margin-bottom: 12px;
		flex-wrap: wrap;
	}

	.order-title h2 {
		margin: 0;
		font-size: 20px;
		color: #1a1a1a;
		flex: 1;
	}

	.status-badge {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		padding: 6px 14px;
		border-radius: 20px;
		font-size: 13px;
		font-weight: 500;
		white-space: nowrap;
	}

	.order-meta {
		display: flex;
		flex-wrap: wrap;
		gap: 16px;
		font-size: 14px;
		color: #666;
	}

	.meta-item {
		display: flex;
		align-items: center;
		gap: 6px;
	}

	.meta-icon {
		font-size: 16px;
	}

	.order-description h4 {
		margin: 0 0 8px 0;
		font-size: 14px;
		color: #666;
		font-weight: 500;
	}

	.order-description p {
		margin: 0;
		line-height: 1.6;
		color: #333;
	}

	.section-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 16px;
		padding-bottom: 12px;
		border-bottom: 1px solid #e5e5e5;
	}

	.section-header h3 {
		margin: 0;
		font-size: 16px;
		color: #1a1a1a;
	}

	.confidence,
	.expert-info {
		font-size: 13px;
		color: #666;
	}

	.diagnosis-summary {
		display: flex;
		align-items: center;
		gap: 12px;
		margin-bottom: 20px;
		flex-wrap: wrap;
	}

	.disease-name {
		display: flex;
		align-items: baseline;
		gap: 8px;
	}

	.disease-label {
		color: #666;
		font-size: 14px;
	}

	.disease-value {
		font-size: 18px;
		font-weight: 600;
		color: #2d5a27;
	}

	.disease-type {
		padding: 2px 8px;
		background-color: #f0f7ef;
		color: #2d5a27;
		border-radius: 4px;
		font-size: 12px;
	}

	.severity-badge {
		padding: 4px 12px;
		border-radius: 20px;
		font-size: 13px;
		font-weight: 500;
		background-color: #fef3c7;
		color: #92400e;
	}

	.info-section {
		margin-bottom: 16px;
	}

	.info-section:last-child {
		margin-bottom: 0;
	}

	.info-section h4 {
		margin: 0 0 6px 0;
		font-size: 14px;
		color: #555;
		font-weight: 500;
	}

	.info-section p {
		margin: 0;
		line-height: 1.6;
		color: #333;
	}

	.rating-display {
		display: flex;
		align-items: center;
		gap: 8px;
		margin-bottom: 20px;
	}

	.star {
		font-size: 24px;
		color: #d1d5db;
	}

	.star.filled {
		color: #fbbf24;
	}

	.rating-text {
		font-size: 14px;
		color: #666;
		margin-left: 8px;
	}

	.solved-status {
		margin-top: 16px;
		padding-top: 16px;
		border-top: 1px solid #e5e5e5;
		font-size: 14px;
	}

	.solved-status .solved {
		color: #16a34a;
		font-weight: 500;
	}

	.solved-status .not-solved {
		color: #dc2626;
		font-weight: 500;
	}

	.feedback-actions {
		background: white;
		border-radius: 12px;
		padding: 20px;
		box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
	}

	.feedback-form h4 {
		margin: 0 0 20px 0;
		font-size: 16px;
		color: #1a1a1a;
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
		min-height: 80px;
	}

	.form-group textarea:focus {
		outline: none;
		border-color: #2d5a27;
	}

	.rating-input {
		display: flex;
		align-items: center;
		gap: 4px;
	}

	.star-btn {
		background: none;
		border: none;
		font-size: 28px;
		color: #d1d5db;
		cursor: pointer;
		padding: 4px;
		transition: color 0.2s;
	}

	.star-btn:hover,
	.star-btn.active {
		color: #fbbf24;
	}

	.rating-value {
		margin-left: 12px;
		font-size: 14px;
		color: #666;
	}

	.checkbox-group label {
		display: flex;
		align-items: center;
		gap: 8px;
		cursor: pointer;
		font-weight: normal;
	}

	.form-actions {
		display: flex;
		gap: 12px;
		margin-top: 24px;
		padding-top: 16px;
		border-top: 1px solid #e5e5e5;
	}

	.btn {
		padding: 10px 24px;
		border-radius: 8px;
		font-size: 14px;
		font-weight: 500;
		cursor: pointer;
		transition: all 0.2s;
		border: none;
		display: inline-flex;
		align-items: center;
		gap: 8px;
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

	.btn-full {
		width: 100%;
		justify-content: center;
	}

	.spinner {
		width: 16px;
		height: 16px;
		border: 2px solid rgba(255, 255, 255, 0.3);
		border-top-color: white;
		border-radius: 50%;
		animation: spin 0.8s linear infinite;
	}

	.chat-section {
		background: white;
		border-radius: 12px;
		overflow: hidden;
		box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
	}

	.chat-section .section-header {
		padding: 16px 20px;
		margin: 0;
		border-bottom: 1px solid #e5e5e5;
	}

	.connection-status {
		font-size: 12px;
	}

	.connection-status.online {
		color: #16a34a;
	}

	.connection-status.offline {
		color: #9ca3af;
	}

	.chat-container {
		display: flex;
		flex-direction: column;
		height: 500px;
	}

	.messages-container {
		flex: 1;
		overflow-y: auto;
		padding: 20px;
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	.empty-messages {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		height: 100%;
		color: #9ca3af;
	}

	.empty-icon {
		font-size: 48px;
		margin-bottom: 12px;
	}

	.message-bubble {
		display: flex;
	}

	.message-bubble.sent {
		justify-content: flex-end;
	}

	.message-bubble.received {
		justify-content: flex-start;
	}

	.message-content {
		max-width: 70%;
		padding: 12px 16px;
		border-radius: 16px;
	}

	.message-bubble.sent .message-content {
		background-color: #2d5a27;
		color: white;
		border-bottom-right-radius: 4px;
	}

	.message-bubble.received .message-content {
		background-color: #f3f4f6;
		color: #1a1a1a;
		border-bottom-left-radius: 4px;
	}

	.message-content p {
		margin: 0 0 4px 0;
		line-height: 1.4;
		word-wrap: break-word;
	}

	.message-image {
		max-width: 200px;
		border-radius: 8px;
		margin-bottom: 4px;
	}

	.message-time {
		font-size: 11px;
		opacity: 0.7;
		display: block;
	}

	.message-input-area {
		display: flex;
		gap: 12px;
		padding: 16px;
		border-top: 1px solid #e5e5e5;
	}

	.message-input-area input {
		flex: 1;
		padding: 12px 16px;
		border: 1px solid #d1d5db;
		border-radius: 20px;
		font-size: 14px;
	}

	.message-input-area input:focus {
		outline: none;
		border-color: #2d5a27;
	}

	.send-btn {
		padding: 12px 24px;
		background-color: #2d5a27;
		color: white;
		border: none;
		border-radius: 20px;
		font-size: 14px;
		font-weight: 500;
		cursor: pointer;
		transition: background-color 0.2s;
	}

	.send-btn:hover:not(:disabled) {
		background-color: #1f431b;
	}

	.send-btn:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	@media (max-width: 640px) {
		.order-title {
			flex-direction: column;
			align-items: flex-start;
		}

		.order-meta {
			flex-direction: column;
			gap: 8px;
		}

		.diagnosis-summary {
			flex-direction: column;
			align-items: flex-start;
		}

		.form-actions {
			flex-direction: column;
		}

		.btn {
			width: 100%;
			justify-content: center;
		}

		.chat-container {
			height: 400px;
		}

		.message-content {
			max-width: 85%;
		}
	}
</style>
