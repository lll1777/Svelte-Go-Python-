<script>
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { auth, isAuthenticated, userRole } from '$stores/auth';
	import { workOrders, statusIcons, statusLabels } from '$stores/workOrders';

	let loading = $state(true);
	let selectedStatus = $state('');
	let searchQuery = $state('');

	$: isFarmer = $userRole === 'farmer';
	$: isExpert = $userRole === 'expert';

	const statusOptions = [
		{ value: '', label: '全部状态' },
		{ value: 'pending', label: '待诊断' },
		{ value: 'diagnosing', label: 'AI诊断中' },
		{ value: 'assigned', label: '已分配专家' },
		{ value: 'consulting', label: '问诊中' },
		{ value: 'prescribed', label: '已开处方' },
		{ value: 'closed', label: '已关闭' }
	];

	$: filteredOrders = $workOrders.workOrders.filter(wo => {
		const matchesStatus = !selectedStatus || wo.status === selectedStatus;
		const matchesSearch = !searchQuery || 
			wo.title.toLowerCase().includes(searchQuery.toLowerCase()) ||
			wo.description.toLowerCase().includes(searchQuery.toLowerCase());
		return matchesStatus && matchesSearch;
	});

	onMount(async () => {
		if (!$isAuthenticated) {
			goto('/login');
			return;
		}

		await workOrders.fetchMyWorkOrders();
		loading = false;
	});

	async function handleStatusChange() {
		await workOrders.fetchMyWorkOrders(selectedStatus || null);
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
			closed: 'bg-gray-100 text-gray-800',
			cancelled: 'bg-red-100 text-red-800'
		};
		return colorMap[status] || 'bg-gray-100 text-gray-800';
	}
</script>

<div class="work-orders-container">
	<header class="page-header">
		<div class="header-left">
			<h1>📋 我的工单</h1>
			<p>查看和管理所有诊断工单</p>
		</div>
		{#if isFarmer}
			<a href="/diagnose" class="btn btn-primary">
				📷 新建诊断
			</a>
		{/if}
	</header>

	<div class="filter-bar">
		<div class="search-box">
			<input
				type="text"
				bind:value={searchQuery}
				placeholder="搜索工单..."
			/>
			<span class="search-icon">🔍</span>
		</div>

		<select
			bind:value={selectedStatus}
			on:change={handleStatusChange}
			class="status-select"
		>
			{#each statusOptions as option}
				<option value={option.value}>{option.label}</option>
			{/each}
		</select>
	</div>

	{#if loading}
		<div class="loading-state">
			<div class="spinner-large"></div>
			<p>加载中...</p>
		</div>
	{:else if filteredOrders.length > 0}
		<div class="orders-list">
			{#each filteredOrders as order}
				<a href="/work-orders/{order.id}" class="order-card">
					<div class="order-main">
						<div class="order-header">
							<h4>{order.title}</h4>
							<span class="status-badge {getStatusColor(order.status)}">
								{statusIcons[order.status]} {statusLabels[order.status]}
							</span>
						</div>
						
						<p class="order-desc">{order.description}</p>
						
						<div class="order-meta">
							<span class="meta-item">
								<span class="meta-label">作物:</span>
								<span class="meta-value">{order.cropType || '未知'}</span>
							</span>
							{#if order.location}
								<span class="meta-item">
									<span class="meta-label">📍</span>
									<span class="meta-value">{order.location}</span>
								</span>
							{/if}
							<span class="meta-item">
								<span class="meta-label">创建时间:</span>
								<span class="meta-value">
									{new Date(order.createdAt).toLocaleDateString('zh-CN', {
										year: 'numeric',
										month: '2-digit',
										day: '2-digit',
										hour: '2-digit',
										minute: '2-digit'
									})}
								</span>
							</span>
						</div>
					</div>

					{#if order.diagnosisResult}
						<div class="diagnosis-preview">
							<span class="ai-badge">AI诊断</span>
							<span class="disease-name">{order.diagnosisResult.diseaseName}</span>
							<span class="confidence">
								({Math.round(order.diagnosisResult.confidence * 100)}%)
							</span>
						</div>
					{/if}

					{#if order.isOfflineCreated}
						<div class="offline-badge">
							📥 离线创建 - 等待同步
						</div>
					{/if}
				</a>
			{/each}
		</div>
	{:else}
		<div class="empty-state">
			<span class="empty-icon">📭</span>
			<h4>暂无工单</h4>
			<p>
				{isFarmer 
					? '点击"新建诊断"开始您的第一次诊断' 
					: '等待农户提交诊断请求'}
			</p>
			{#if isFarmer}
				<a href="/diagnose" class="btn btn-primary">
					开始诊断
				</a>
			{/if}
		</div>
	{/if}
</div>

<style>
	.work-orders-container {
		max-width: 900px;
		margin: 0 auto;
	}

	.page-header {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		margin-bottom: 24px;
	}

	.header-left h1 {
		font-size: 24px;
		margin: 0 0 8px 0;
	}

	.header-left p {
		margin: 0;
		color: #666;
		font-size: 14px;
	}

	.filter-bar {
		display: flex;
		gap: 16px;
		margin-bottom: 24px;
		flex-wrap: wrap;
	}

	.search-box {
		flex: 1;
		min-width: 200px;
		position: relative;
	}

	.search-box input {
		width: 100%;
		padding: 12px 16px 12px 44px;
		border: 1px solid #d1d5db;
		border-radius: 8px;
		font-size: 14px;
		box-sizing: border-box;
	}

	.search-box input:focus {
		outline: none;
		border-color: #2d5a27;
	}

	.search-icon {
		position: absolute;
		left: 14px;
		top: 50%;
		transform: translateY(-50%);
		font-size: 16px;
	}

	.status-select {
		padding: 12px 16px;
		border: 1px solid #d1d5db;
		border-radius: 8px;
		font-size: 14px;
		background: white;
		min-width: 140px;
	}

	.status-select:focus {
		outline: none;
		border-color: #2d5a27;
	}

	.orders-list {
		display: flex;
		flex-direction: column;
		gap: 16px;
	}

	.order-card {
		background: white;
		border-radius: 12px;
		padding: 20px;
		text-decoration: none;
		color: inherit;
		transition: all 0.2s;
		box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	.order-card:hover {
		transform: translateY(-2px);
		box-shadow: 0 4px 16px rgba(0, 0, 0, 0.1);
	}

	.order-header {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		gap: 16px;
	}

	.order-header h4 {
		margin: 0;
		font-size: 16px;
		color: #1a1a1a;
		flex: 1;
	}

	.status-badge {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		padding: 4px 12px;
		border-radius: 20px;
		font-size: 12px;
		font-weight: 500;
		white-space: nowrap;
	}

	.order-desc {
		margin: 0;
		font-size: 14px;
		color: #666;
		display: -webkit-box;
		-webkit-line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
	}

	.order-meta {
		display: flex;
		flex-wrap: wrap;
		gap: 16px;
		font-size: 13px;
	}

	.meta-item {
		display: flex;
		align-items: center;
		gap: 4px;
	}

	.meta-label {
		color: #888;
	}

	.meta-value {
		color: #555;
		font-weight: 500;
	}

	.diagnosis-preview {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 10px 12px;
		background-color: #f0f7ef;
		border-radius: 8px;
		font-size: 13px;
	}

	.ai-badge {
		padding: 2px 8px;
		background-color: #2d5a27;
		color: white;
		border-radius: 4px;
		font-size: 11px;
		font-weight: 500;
	}

	.disease-name {
		font-weight: 500;
		color: #2d5a27;
	}

	.confidence {
		color: #666;
		margin-left: auto;
	}

	.offline-badge {
		padding: 8px 12px;
		background-color: #fef3c7;
		color: #92400e;
		border-radius: 8px;
		font-size: 13px;
		font-weight: 500;
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

	.empty-state {
		text-align: center;
		padding: 60px 20px;
		background: white;
		border-radius: 12px;
	}

	.empty-icon {
		font-size: 56px;
		display: block;
		margin-bottom: 20px;
	}

	.empty-state h4 {
		margin: 0 0 8px 0;
		font-size: 18px;
		color: #1a1a1a;
	}

	.empty-state p {
		margin: 0 0 24px 0;
		font-size: 14px;
		color: #666;
	}

	@media (max-width: 640px) {
		.page-header {
			flex-direction: column;
			gap: 16px;
		}

		.filter-bar {
			flex-direction: column;
		}

		.search-box,
		.status-select {
			width: 100%;
		}

		.order-meta {
			flex-direction: column;
			gap: 8px;
		}

		.diagnosis-preview {
			flex-wrap: wrap;
		}
	}
</style>
