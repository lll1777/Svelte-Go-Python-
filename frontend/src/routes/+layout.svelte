<script>
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { auth, isAuthenticated } from '$stores/auth';
	import { wsService } from '$lib/services/websocket';
	import { offlineStorage } from '$lib/utils/offlineStorage';

	let isOnline = $state(navigator.onLine);
	let showMenu = $state(false);

	$: currentPath = $page.url.pathname;
	$: userRole = $auth.user?.role;
	$: isFarmer = userRole === 'farmer';
	$: isExpert = userRole === 'expert';

	onMount(() => {
		offlineStorage.initDB();

		const handleOnline = () => {
			isOnline = true;
			offlineStorage.processSyncQueue();
		};

		const handleOffline = () => {
			isOnline = false;
		};

		window.addEventListener('online', handleOnline);
		window.addEventListener('offline', handleOffline);

		if ($isAuthenticated) {
			auth.fetchProfile();
			wsService.connect();
		}

		return () => {
			window.removeEventListener('online', handleOnline);
			window.removeEventListener('offline', handleOffline);
			wsService.disconnect();
		};
	});

	$: if ($isAuthenticated) {
		wsService.connect();
	}

	const navItems = [
		{ path: '/', label: '首页', icon: '🏠', roles: ['farmer', 'expert', 'admin'] },
		{ path: '/diagnose', label: '拍照诊断', icon: '📷', roles: ['farmer'] },
		{ path: '/work-orders', label: '我的工单', icon: '📋', roles: ['farmer', 'expert'] },
		{ path: '/expert/workbench', label: '专家工作台', icon: '💼', roles: ['expert'] },
		{ path: '/profile', label: '个人中心', icon: '👤', roles: ['farmer', 'expert', 'admin'] }
	];

	const filteredNavItems = navItems.filter(item => 
		!userRole || item.roles.includes(userRole)
	);
</script>

<div class="app-container">
	<header class="header">
		<div class="header-content">
			<div class="logo">
				<span class="logo-icon">🌾</span>
				<span class="logo-text">智能农业服务平台</span>
			</div>
			
			{#if $isAuthenticated}
				<nav class="desktop-nav">
					{#each filteredNavItems as item}
						<a href={item.path} class="nav-link {currentPath === item.path ? 'active' : ''}">
							<span class="nav-icon">{item.icon}</span>
							<span class="nav-label">{item.label}</span>
						</a>
					{/each}
				</nav>

				<div class="header-right">
					<div class="network-status" class:online={isOnline}>
						<span class="status-dot"></span>
						<span class="status-text">{isOnline ? '在线' : '离线'}</span>
					</div>
					
					<div class="user-menu">
						<button class="user-btn" on:click={() => showMenu = !showMenu}>
							<div class="user-avatar">
								{$auth.user?.avatar || '👤'}
							</div>
							<span class="user-name">{$auth.user?.full_name || '用户'}</span>
						</button>
						
						{#if showMenu}
							<div class="dropdown-menu">
								<a href="/profile" class="dropdown-item">
									<span>👤</span> 个人中心
								</a>
								<div class="dropdown-divider"></div>
								<button class="dropdown-item" on:click={() => auth.logout()}>
									<span>🚪</span> 退出登录
								</button>
							</div>
						{/if}
					</div>
				</div>
			{/if}
		</div>
	</header>

	{#if $isAuthenticated}
		<nav class="mobile-nav">
			{#each filteredNavItems as item}
				<a href={item.path} class="mobile-nav-link {currentPath === item.path ? 'active' : ''}">
					<span class="mobile-nav-icon">{item.icon}</span>
					<span class="mobile-nav-label">{item.label}</span>
				</a>
			{/each}
		</nav>
	{/if}

	<main class="main-content">
		<slot />
	</main>

	{#if !isOnline}
		<div class="offline-banner">
			<span>⚠️ 当前处于离线模式，部分功能受限。数据将在联网后自动同步。</span>
		</div>
	{/if}
</div>

<style>
	.app-container {
		min-height: 100vh;
		display: flex;
		flex-direction: column;
		background-color: #f5f7fa;
	}

	.header {
		background: linear-gradient(135deg, #2d5a27 0%, #4a7c44 100%);
		color: white;
		box-shadow: 0 2px 10px rgba(0, 0, 0, 0.1);
		position: sticky;
		top: 0;
		z-index: 100;
	}

	.header-content {
		max-width: 1400px;
		margin: 0 auto;
		padding: 0 20px;
		height: 60px;
		display: flex;
		align-items: center;
		justify-content: space-between;
	}

	.logo {
		display: flex;
		align-items: center;
		gap: 10px;
	}

	.logo-icon {
		font-size: 28px;
	}

	.logo-text {
		font-size: 18px;
		font-weight: 600;
	}

	.desktop-nav {
		display: flex;
		gap: 5px;
	}

	@media (max-width: 768px) {
		.desktop-nav {
			display: none;
		}
	}

	.nav-link {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 8px 16px;
		border-radius: 8px;
		color: rgba(255, 255, 255, 0.9);
		text-decoration: none;
		transition: all 0.2s;
	}

	.nav-link:hover {
		background-color: rgba(255, 255, 255, 0.15);
		color: white;
	}

	.nav-link.active {
		background-color: rgba(255, 255, 255, 0.2);
		color: white;
		font-weight: 500;
	}

	.header-right {
		display: flex;
		align-items: center;
		gap: 15px;
	}

	.network-status {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 4px 12px;
		border-radius: 20px;
		background-color: rgba(255, 255, 255, 0.15);
		font-size: 12px;
	}

	.network-status.online .status-dot {
		background-color: #4ade80;
	}

	.network-status:not(.online) .status-dot {
		background-color: #fbbf24;
	}

	.status-dot {
		width: 8px;
		height: 8px;
		border-radius: 50%;
	}

	.user-menu {
		position: relative;
	}

	.user-btn {
		display: flex;
		align-items: center;
		gap: 8px;
		background: none;
		border: none;
		color: white;
		cursor: pointer;
		padding: 6px 10px;
		border-radius: 8px;
		transition: background 0.2s;
	}

	.user-btn:hover {
		background-color: rgba(255, 255, 255, 0.15);
	}

	.user-avatar {
		width: 32px;
		height: 32px;
		border-radius: 50%;
		background-color: rgba(255, 255, 255, 0.2);
		display: flex;
		align-items: center;
		justify-content: center;
		font-size: 16px;
	}

	.user-name {
		font-size: 14px;
		font-weight: 500;
	}

	.dropdown-menu {
		position: absolute;
		top: 100%;
		right: 0;
		margin-top: 5px;
		background: white;
		border-radius: 8px;
		box-shadow: 0 4px 20px rgba(0, 0, 0, 0.15);
		min-width: 160px;
		overflow: hidden;
		z-index: 1000;
	}

	.dropdown-item {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 12px 16px;
		color: #333;
		text-decoration: none;
		border: none;
		background: none;
		width: 100%;
		cursor: pointer;
		transition: background 0.2s;
		text-align: left;
	}

	.dropdown-item:hover {
		background-color: #f5f5f5;
	}

	.dropdown-divider {
		height: 1px;
		background-color: #e5e5e5;
		margin: 4px 0;
	}

	.mobile-nav {
		display: none;
		background: white;
		border-top: 1px solid #e5e5e5;
		position: fixed;
		bottom: 0;
		left: 0;
		right: 0;
		z-index: 100;
		padding: 8px 0;
	}

	@media (max-width: 768px) {
		.mobile-nav {
			display: flex;
			justify-content: space-around;
		}
	}

	.mobile-nav-link {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 4px;
		padding: 6px 12px;
		color: #666;
		text-decoration: none;
		font-size: 12px;
	}

	.mobile-nav-link.active {
		color: #2d5a27;
	}

	.mobile-nav-icon {
		font-size: 20px;
	}

	.main-content {
		flex: 1;
		padding: 20px;
		max-width: 1400px;
		margin: 0 auto;
		width: 100%;
		padding-bottom: 80px;
	}

	@media (max-width: 768px) {
		.main-content {
			padding: 15px;
			padding-bottom: 90px;
		}
	}

	.offline-banner {
		position: fixed;
		bottom: 70px;
		left: 50%;
		transform: translateX(-50%);
		background-color: #fef3c7;
		color: #92400e;
		padding: 10px 20px;
		border-radius: 8px;
		font-size: 13px;
		box-shadow: 0 2px 10px rgba(0, 0, 0, 0.1);
		z-index: 99;
	}

	@media (min-width: 769px) {
		.offline-banner {
			bottom: 20px;
		}
	}
</style>
