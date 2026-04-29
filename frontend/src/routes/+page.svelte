<script>
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { auth, isAuthenticated, userRole } from '$stores/auth';
	import { workOrders, statusIcons, statusLabels } from '$stores/workOrders';

	let loading = $state(true);

	$: isFarmer = $userRole === 'farmer';
	$: isExpert = $userRole === 'expert';

	onMount(async () => {
		if (!$isAuthenticated) {
			goto('/login');
			return;
		}

		await auth.fetchProfile();
		await workOrders.fetchMyWorkOrders();
		loading = false;
	});

	const quickActions = [
		{
			icon: '📷',
			label: '拍照诊断',
			description: '拍摄作物照片，AI自动识别病虫害',
			path: '/diagnose',
			showFor: ['farmer']
		},
		{
			icon: '📋',
			label: '我的工单',
			description: '查看和管理所有诊断工单',
			path: '/work-orders',
			showFor: ['farmer', 'expert']
		},
		{
			icon: '💬',
			label: '在线问诊',
			description: '与农技专家实时交流',
			path: '/work-orders',
			showFor: ['farmer', 'expert']
		},
		{
			icon: '👨‍🔬',
			label: '专家工作台',
			description: '管理待诊断工单和问诊',
			path: '/expert/workbench',
			showFor: ['expert']
		}
	];

	$: filteredActions = quickActions.filter(action => 
		action.showFor.includes($userRole || 'farmer')
	);

	$: recentOrders = $workOrders.workOrders.slice(0, 5);
</script>

<div class="home-container">
	{#if !loading}
		<section class="welcome-section">
			<div class="welcome-content">
				<div class="greeting">
					<h2>你好，{$auth.user?.full_name || '用户'}</h2>
					<p class="role-badge">
						{isFarmer ? '🌾 种植户' : '👨‍🔬 农技专家'}
					</p>
				</div>
				<div class="stats">
					<div class="stat-card">
						<span class="stat-value">{recentOrders.length}</span>
						<span class="stat-label">总工单</span>
					</div>
					<div class="stat-card pending">
						<span class="stat-value">
							{recentOrders.filter(wo => 
								['pending', 'diagnosing', 'assigned', 'consulting'].includes(wo.status)
							).length}
						</span>
						<span class="stat-label">进行中</span>
					</div>
					<div class="stat-card completed">
						<span class="stat-value">
							{recentOrders.filter(wo => 
								['prescribed', 'confirmed', 'closed'].includes(wo.status)
							).length}
						</span>
						<span class="stat-label">已完成</span>
					</div>
				</div>
			</div>
		</section>

		<section class="quick-actions">
			<h3>快捷操作</h3>
			<div class="actions-grid">
				{#each filteredActions as action}
					<a href={action.path} class="action-card">
						<span class="action-icon">{action.icon}</span>
						<div class="action-info">
							<h4>{action.label}</h4>
							<p>{action.description}</p>
						</div>
						<span class="action-arrow">→</span>
					</a>
				{/each}
			</div>
		</section>

		<section class="recent-orders">
			<div class="section-header">
				<h3>最近工单</h3>
				<a href="/work-orders" class="view-all">查看全部 →</a>
			</div>

			{#if recentOrders.length > 0}
				<div class="orders-list">
					{#each recentOrders as order}
						<a href="/work-orders/{order.id}" class="order-card">
							<div class="order-main">
								<div class="order-header">
									<h4>{order.title}</h4>
									<span class="status-badge {order.status}">
										{statusIcons[order.status]} {statusLabels[order.status]}
									</span>
								</div>
								<p class="order-desc">{order.description}</p>
								<div class="order-meta">
									<span class="crop-type">🌱 {order.cropType || '未知作物'}</span>
									<span class="order-time">
										{new Date(order.createdAt).toLocaleDateString('zh-CN')}
									</span>
								</div>
							</div>
							{#if order.diagnosisResult}
								<div class="diagnosis-preview">
									<span class="ai-badge">AI诊断</span>
									<span>{order.diagnosisResult.diseaseName}</span>
									<span class="confidence">
										({Math.round(order.diagnosisResult.confidence * 100)}%)
									</span>
								</div>
							{/if}
						</a>
					{/each}
				</div>
			{:else}
				<div class="empty-state">
					<span class="empty-icon">📭</span>
					<h4>暂无工单</h4>
					<p>{isFarmer ? '点击"拍照诊断"开始您的第一次诊断' : '等待农户提交诊断请求'}</p>
					{#if isFarmer}
						<a href="/diagnose" class="btn btn-primary">开始诊断</a>
					{/if}
				</div>
			{/if}
		</section>
	{:else}
		<div class="loading-state">
			<div class="spinner-large"></div>
			<p>加载中...</p>
		</div>
	{/if}
</div>

<style>
	.home-container {
		display: flex;
		flex-direction: column;
		gap: 24px;
	}

	.welcome-section {
		background: linear-gradient(135deg, #2d5a27 0%, #4a7c44 100%);
		border-radius: 16px;
		padding: 24px;
		color: white;
	}

	.welcome-content {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		flex-wrap: wrap;
		gap: 20px;
	}

	.greeting h2 {
		font-size: 24px;
		margin: 0 0 8px 0;
	}

	.role-badge {
		display: inline-block;
		padding: 6px 12px;
		background-color: rgba(255, 255, 255, 0.2);
		border-radius: 20px;
		font-size: 14px;
	}

	.stats {
		display: flex;
		gap: 20px;
	}

	.stat-card {
		text-align: center;
		padding: 12px 20px;
		background-color: rgba(255, 255, 255, 0.1);
		border-radius: 12px;
		min-width: 80px;
	}

	.stat-card.pending {
		background-color: rgba(251, 191, 36, 0.2);
	}

	.stat-card.completed {
		background-color: rgba(74, 222, 128, 0.2);
	}

	.stat-value {
		display: block;
		font-size: 28px;
		font-weight: 600;
	}

	.stat-label {
		display: block;
		font-size: 12px;
		opacity: 0.9;
		margin-top: 4px;
	}

	@media (max-width: 640px) {
		.welcome-content {
			flex-direction: column;
		}

		.stats {
			width: 100%;
			justify-content: space-between;
		}

		.stat-card {
			flex: 1;
			padding: 10px;
		}

		.stat-value {
			font-size: 24px;
		}
	}

	.quick-actions h3,
	.recent-orders h3 {
		font-size: 18px;
		font-weight: 600;
		margin: 0 0 16px 0;
		color: #1a1a1a;
	}

	.actions-grid {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
		gap: 16px;
	}

	.action-card {
		display: flex;
		align-items: center;
		gap: 16px;
		padding: 20px;
		background: white;
		border-radius: 12px;
		text-decoration: none;
		color: inherit;
		transition: all 0.2s;
		box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
	}

	.action-card:hover {
		transform: translateY(-2px);
		box-shadow: 0 4px 16px rgba(0, 0, 0, 0.1);
	}

	.action-icon {
		font-size: 36px;
	}

	.action-info {
		flex: 1;
	}

	.action-info h4 {
		margin: 0 0 4px 0;
		font-size: 16px;
		color: #1a1a1a;
	}

	.action-info p {
		margin: 0;
		font-size: 13px;
		color: #666;
	}

	.action-arrow {
		font-size: 20px;
		color: #999;
	}

	.section-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 16px;
	}

	.view-all {
		color: #2d5a27;
		text-decoration: none;
		font-size: 14px;
		font-weight: 500;
	}

	.view-all:hover {
		text-decoration: underline;
	}

	.orders-list {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	.order-card {
		background: white;
		border-radius: 12px;
		padding: 16px;
		text-decoration: none;
		color: inherit;
		transition: all 0.2s;
		box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
	}

	.order-card:hover {
		transform: translateY(-1px);
		box-shadow: 0 4px 16px rgba(0, 0, 0, 0.1);
	}

	.order-header {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		gap: 12px;
		margin-bottom: 8px;
	}

	.order-header h4 {
		margin: 0;
		font-size: 15px;
		color: #1a1a1a;
		flex: 1;
	}

	.status-badge {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		padding: 4px 10px;
		border-radius: 20px;
		font-size: 12px;
		font-weight: 500;
		white-space: nowrap;
	}

	.status-badge.pending,
	.status-badge.diagnosing {
		background-color: #fef3c7;
		color: #92400e;
	}

	.status-badge.assigned,
	.status-badge.consulting {
		background-color: #dbeafe;
		color: #1e40af;
	}

	.status-badge.prescribed,
	.status-badge.confirmed {
		background-color: #dcfce7;
		color: #166534;
	}

	.status-badge.closed {
		background-color: #f3f4f6;
		color: #4b5563;
	}

	.order-desc {
		margin: 0 0 12px 0;
		font-size: 14px;
		color: #666;
		display: -webkit-box;
		-webkit-line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
	}

	.order-meta {
		display: flex;
		gap: 16px;
		font-size: 13px;
		color: #888;
	}

	.diagnosis-preview {
		margin-top: 12px;
		padding: 10px 12px;
		background-color: #f0f7ef;
		border-radius: 8px;
		font-size: 13px;
		display: flex;
		align-items: center;
		gap: 8px;
	}

	.ai-badge {
		padding: 2px 8px;
		background-color: #2d5a27;
		color: white;
		border-radius: 4px;
		font-size: 11px;
		font-weight: 500;
	}

	.confidence {
		color: #666;
		margin-left: auto;
	}

	.empty-state {
		text-align: center;
		padding: 40px 20px;
		background: white;
		border-radius: 12px;
	}

	.empty-icon {
		font-size: 48px;
		display: block;
		margin-bottom: 16px;
	}

	.empty-state h4 {
		margin: 0 0 8px 0;
		font-size: 16px;
		color: #1a1a1a;
	}

	.empty-state p {
		margin: 0 0 20px 0;
		font-size: 14px;
		color: #666;
	}

	.btn {
		padding: 10px 24px;
		border-radius: 8px;
		font-size: 14px;
		font-weight: 500;
		cursor: pointer;
		transition: all 0.2s;
		border: none;
		display: inline-block;
		text-decoration: none;
	}

	.btn-primary {
		background-color: #2d5a27;
		color: white;
	}

	.btn-primary:hover {
		background-color: #1f431b;
	}

	.loading-state {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		padding: 80px 20px;
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
</style>
