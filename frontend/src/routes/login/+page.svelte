<script>
	import { goto } from '$app/navigation';
	import { auth } from '$stores/auth';

	let username = $state('');
	let password = $state('');
	let isRegisterMode = $state(false);
	let isLoading = $state(false);
	let error = $state('');

	let registerData = $state({
		fullName: '',
		phone: '',
		passwordConfirm: '',
		role: 'farmer'
	});

	async function handleSubmit(e) {
		e.preventDefault();
		error = '';
		isLoading = true;

		try {
			if (isRegisterMode) {
				if (password !== registerData.passwordConfirm) {
					throw new Error('两次输入的密码不一致');
				}

				const result = await auth.register({
					username,
					password,
					full_name: registerData.fullName,
					phone: registerData.phone,
					role: registerData.role
				});

				if (result.success) {
					error = '注册成功，请登录';
					isRegisterMode = false;
				} else {
					throw new Error(result.error);
				}
			} else {
				const result = await auth.login(username, password);
				if (result.success) {
					goto('/');
				} else {
					throw new Error(result.error);
				}
			}
		} catch (err) {
			error = err.message;
		} finally {
			isLoading = false;
		}
	}

	function toggleMode() {
		isRegisterMode = !isRegisterMode;
		error = '';
	}
</script>

<div class="auth-container">
	<div class="auth-card">
		<div class="auth-header">
			<div class="logo-large">🌾</div>
			<h1>智能农业服务平台</h1>
			<p class="subtitle">
				{isRegisterMode ? '创建您的账户' : '登录您的账户'}
			</p>
		</div>

		{#if error}
			<div class="alert alert-error">
				{error}
			</div>
		{/if}

		<form on:submit={handleSubmit} class="auth-form">
			<div class="form-group">
				<label for="username">用户名</label>
				<input
					id="username"
					type="text"
					bind:value={username}
					placeholder="请输入用户名"
					required
					disabled={isLoading}
				/>
			</div>

			{#if isRegisterMode}
				<div class="form-group">
					<label for="fullName">姓名</label>
					<input
						id="fullName"
						type="text"
						bind:value={registerData.fullName}
						placeholder="请输入真实姓名"
						required
						disabled={isLoading}
					/>
				</div>

				<div class="form-group">
					<label for="phone">手机号</label>
					<input
						id="phone"
						type="tel"
						bind:value={registerData.phone}
						placeholder="请输入手机号"
						required
						disabled={isLoading}
					/>
				</div>

				<div class="form-group">
					<label>用户类型</label>
					<div class="role-selector">
						<label class="role-option">
							<input
								type="radio"
								name="role"
								value="farmer"
								bind:group={registerData.role}
								disabled={isLoading}
							/>
							<span>🌾 种植户</span>
						</label>
						<label class="role-option">
							<input
								type="radio"
								name="role"
								value="expert"
								bind:group={registerData.role}
								disabled={isLoading}
							/>
							<span>👨‍🔬 农技专家</span>
						</label>
					</div>
				</div>
			{/if}

			<div class="form-group">
				<label for="password">密码</label>
				<input
					id="password"
					type="password"
					bind:value={password}
					placeholder="请输入密码"
					required
					disabled={isLoading}
				/>
			</div>

			{#if isRegisterMode}
				<div class="form-group">
					<label for="passwordConfirm">确认密码</label>
					<input
						id="passwordConfirm"
						type="password"
						bind:value={registerData.passwordConfirm}
						placeholder="请再次输入密码"
						required
						disabled={isLoading}
					/>
				</div>
			{/if}

			<button
				type="submit"
				class="btn btn-primary btn-full"
				disabled={isLoading}
			>
				{#if isLoading}
					<span class="spinner"></span>
					<span>处理中...</span>
				{:else}
					{isRegisterMode ? '注册' : '登录'}
				{/if}
			</button>
		</form>

		<div class="auth-footer">
			<span>
				{isRegisterMode ? '已有账户？' : '还没有账户？'}
			</span>
			<button type="button" class="btn-link" on:click={toggleMode}>
				{isRegisterMode ? '立即登录' : '立即注册'}
			</button>
		</div>

		<div class="demo-info">
			<h4>演示账户</h4>
			<div class="demo-accounts">
				<div class="demo-account">
					<span class="role-badge farmer">种植户</span>
					<span>farmer1 / password123</span>
				</div>
				<div class="demo-account">
					<span class="role-badge expert">专家</span>
					<span>expert1 / password123</span>
				</div>
			</div>
		</div>
	</div>
</div>

<style>
	.auth-container {
		min-height: 100vh;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 20px;
		background: linear-gradient(135deg, #2d5a27 0%, #4a7c44 50%, #6b9e63 100%);
	}

	.auth-card {
		background: white;
		border-radius: 16px;
		padding: 40px;
		width: 100%;
		max-width: 450px;
		box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
	}

	.auth-header {
		text-align: center;
		margin-bottom: 30px;
	}

	.logo-large {
		font-size: 60px;
		display: block;
		margin-bottom: 10px;
	}

	.auth-header h1 {
		font-size: 24px;
		color: #1a1a1a;
		margin-bottom: 8px;
	}

	.subtitle {
		color: #666;
		font-size: 14px;
	}

	.alert {
		padding: 12px 16px;
		border-radius: 8px;
		margin-bottom: 20px;
		font-size: 14px;
	}

	.alert-error {
		background-color: #fef2f2;
		color: #dc2626;
		border: 1px solid #fecaca;
	}

	.auth-form {
		display: flex;
		flex-direction: column;
		gap: 16px;
	}

	.form-group {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.form-group label {
		font-size: 14px;
		font-weight: 500;
		color: #374151;
	}

	.form-group input {
		padding: 12px 16px;
		border: 1px solid #d1d5db;
		border-radius: 8px;
		font-size: 14px;
		transition: border-color 0.2s, box-shadow 0.2s;
	}

	.form-group input:focus {
		outline: none;
		border-color: #4a7c44;
		box-shadow: 0 0 0 3px rgba(74, 124, 68, 0.1);
	}

	.form-group input:disabled {
		background-color: #f3f4f6;
	}

	.role-selector {
		display: flex;
		gap: 16px;
	}

	.role-option {
		display: flex;
		align-items: center;
		gap: 6px;
		cursor: pointer;
		padding: 10px 16px;
		border: 1px solid #d1d5db;
		border-radius: 8px;
		transition: all 0.2s;
	}

	.role-option:hover {
		border-color: #4a7c44;
	}

	.role-option:has(input:checked) {
		border-color: #4a7c44;
		background-color: #f0f7ef;
	}

	.role-option input {
		margin: 0;
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
		justify-content: center;
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

	.btn-full {
		width: 100%;
	}

	.btn-link {
		background: none;
		border: none;
		color: #2d5a27;
		font-weight: 500;
		cursor: pointer;
		padding: 0;
	}

	.btn-link:hover {
		text-decoration: underline;
	}

	.spinner {
		width: 16px;
		height: 16px;
		border: 2px solid rgba(255, 255, 255, 0.3);
		border-top-color: white;
		border-radius: 50%;
		animation: spin 0.8s linear infinite;
	}

	@keyframes spin {
		to { transform: rotate(360deg); }
	}

	.auth-footer {
		margin-top: 24px;
		text-align: center;
		font-size: 14px;
		color: #666;
	}

	.demo-info {
		margin-top: 30px;
		padding-top: 20px;
		border-top: 1px solid #e5e5e5;
	}

	.demo-info h4 {
		font-size: 13px;
		color: #666;
		margin-bottom: 10px;
		font-weight: 500;
	}

	.demo-accounts {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.demo-account {
		display: flex;
		align-items: center;
		gap: 10px;
		font-size: 13px;
		color: #555;
		font-family: monospace;
	}

	.role-badge {
		padding: 2px 8px;
		border-radius: 4px;
		font-size: 11px;
		font-weight: 500;
	}

	.role-badge.farmer {
		background-color: #dbeafe;
		color: #1e40af;
	}

	.role-badge.expert {
		background-color: #fce7f3;
		color: #9d174d;
	}
</style>
