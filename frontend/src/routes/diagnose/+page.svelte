<script>
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { auth, isAuthenticated } from '$stores/auth';
	import { workOrders } from '$stores/workOrders';
	import { offlineStorage } from '$lib/utils/offlineStorage';

	let step = $state(1);
	let selectedImages = $state([]);
	let previewImages = $state([]);
	let isSubmitting = $state(false);
	let error = $state('');
	let diagnosisResult = $state(null);
	let workOrderId = $state(null);

	let formData = $state({
		title: '',
		description: '',
		cropType: 'rice',
		location: '',
		latitude: null,
		longitude: null
	});

	const cropOptions = [
		{ value: 'rice', label: '🌾 水稻' },
		{ value: 'vegetable', label: '🥬 蔬菜' },
		{ value: 'fruit_tree', label: '🍎 果树' },
		{ value: 'other', label: '🌱 其他' }
	];

	$: canProceed = selectedImages.length > 0 && formData.title.trim();
	$: isOnline = navigator.onLine;

	onMount(() => {
		if (!$isAuthenticated) {
			goto('/login');
			return;
		}

		getLocation();
	});

	async function getLocation() {
		if ('geolocation' in navigator) {
			try {
				const position = await new Promise((resolve, reject) => {
					navigator.geolocation.getCurrentPosition(resolve, reject, {
						enableHighAccuracy: true,
						timeout: 5000,
						maximumAge: 0
					});
				});

				formData.latitude = position.coords.latitude;
				formData.longitude = position.coords.longitude;
			} catch (err) {
				console.log('Location access denied or unavailable');
			}
		}
	}

	function handleImageSelect(e) {
		const files = Array.from(e.target.files);
		
		files.forEach(file => {
			if (selectedImages.length >= 5) return;
			
			if (!file.type.startsWith('image/')) return;
			if (file.size > 10 * 1024 * 1024) return;

			const reader = new FileReader();
			reader.onload = (event) => {
				previewImages = [...previewImages, {
					file,
					preview: event.target.result,
					name: file.name
				}];
			};
			reader.readAsDataURL(file);

			selectedImages = [...selectedImages, file];
		});
	}

	function removeImage(index) {
		previewImages = previewImages.filter((_, i) => i !== index);
		selectedImages = selectedImages.filter((_, i) => i !== index);
	}

	async function submitDiagnosis() {
		if (selectedImages.length === 0) {
			error = '请至少选择一张图片';
			return;
		}

		if (!formData.title.trim()) {
			error = '请输入问题描述标题';
			return;
		}

		isSubmitting = true;
		error = '';

		try {
			if (!isOnline) {
				await saveOfflineDiagnosis();
				step = 4;
				return;
			}

			const formDataObj = new FormData();
			
			selectedImages.forEach((file, index) => {
				formDataObj.append('images', file);
			});

			formDataObj.append('title', formData.title);
			formDataObj.append('description', formData.description);
			formDataObj.append('crop_type', formData.cropType);
			if (formData.location) {
				formDataObj.append('location', formData.location);
			}
			if (formData.latitude) {
				formDataObj.append('latitude', formData.latitude.toString());
			}
			if (formData.longitude) {
				formDataObj.append('longitude', formData.longitude.toString());
			}

			const result = await workOrders.uploadAndDiagnose(formDataObj);

			if (result.success) {
				workOrderId = result.workOrder.id;
				diagnosisResult = result.workOrder.diagnosis_result;
				step = 3;
			} else {
				throw new Error(result.error);
			}

		} catch (err) {
			error = err.message || '诊断提交失败，请重试';
		} finally {
			isSubmitting = false;
		}
	}

	async function saveOfflineDiagnosis() {
		const offlineWo = {
			title: formData.title,
			description: formData.description,
			cropType: formData.cropType,
			location: formData.location,
			latitude: formData.latitude,
			longitude: formData.longitude,
			isOfflineCreated: true,
			offlineSyncStatus: 'pending'
		};

		await workOrders.createWorkOrder(offlineWo);

		for (let i = 0; i < previewImages.length; i++) {
			const preview = previewImages[i];
			const hash = await generateImageHash(preview.preview);
			await offlineStorage.saveImage(hash, preview.preview);
		}
	}

	async function generateImageHash(dataUrl) {
		const hashBuffer = await crypto.subtle.digest(
			'SHA-256',
			new TextEncoder().encode(dataUrl.slice(0, 10000))
		);
		const hashArray = Array.from(new Uint8Array(hashBuffer));
		return hashArray.map(b => b.toString(16).padStart(2, '0')).join('');
	}

	function nextStep() {
		if (step === 1 && canProceed) {
			step = 2;
		}
	}

	function prevStep() {
		if (step > 1) {
			step--;
		}
	}

	function viewWorkOrder() {
		if (workOrderId) {
			goto(`/work-orders/${workOrderId}`);
		}
	}

	const severityColors = {
		'轻度': 'bg-green-100 text-green-800',
		'中度': 'bg-yellow-100 text-yellow-800',
		'重度': 'bg-red-100 text-red-800'
	};
</script>

<div class="diagnose-container">
	<header class="page-header">
		<h1>📷 拍照诊断</h1>
		<p>上传作物照片，AI智能识别病虫害</p>
	</header>

	{#if step <= 3}
		<div class="steps-indicator">
			{#each ['选择图片', '填写信息', '诊断结果'] as label, i}
				<div class="step-item {i + 1 === step ? 'active' : ''} {i + 1 < step ? 'completed' : ''}">
					<span class="step-number">
						{#if i + 1 < step}✓{:else}{i + 1}{/if}
					</span>
					<span class="step-label">{label}</span>
				</div>
				{#if i < 2}
					<div class="step-line {i + 1 < step ? 'completed' : ''}"></div>
				{/if}
			{/each}
		</div>
	{/if}

	{#if error}
		<div class="alert alert-error">
			{error}
		</div>
	{/if}

	{#if !isOnline && step <= 2}
		<div class="alert alert-warning">
			⚠️ 当前处于离线模式，诊断数据将在联网后自动同步
		</div>
	{/if}

	{#if step === 1}
		<div class="step-content">
			<section class="image-upload-section">
				<h3>选择图片 ({selectedImages.length}/5)</h3>
				
				<div class="upload-area" on:click|preventDefault={() => document.getElementById('file-input').click()}>
					<input
						id="file-input"
						type="file"
						accept="image/*"
						multiple
						on:change={handleImageSelect}
						style="display: none;"
					/>
					<span class="upload-icon">📷</span>
					<p>点击或拖拽图片到此处</p>
					<p class="upload-hint">支持 JPG、PNG 格式，单张不超过 10MB</p>
				</div>

				{#if previewImages.length > 0}
					<div class="preview-grid">
						{#each previewImages as img, index}
							<div class="preview-item">
								<img src={img.preview} alt={img.name} />
								<button
									type="button"
									class="remove-btn"
									on:click={() => removeImage(index)}
								>✕</button>
							</div>
						{/each}
					</div>
				{/if}
			</section>

			<div class="form-actions">
				<a href="/" class="btn btn-secondary">取消</a>
				<button
					type="button"
					class="btn btn-primary"
					disabled={!canProceed || isSubmitting}
					on:click={nextStep}
				>
					下一步 →
				</button>
			</div>
		</div>
	{:else if step === 2}
		<div class="step-content">
			<section class="form-section">
				<h3>填写诊断信息</h3>

				<div class="form-group">
					<label for="title">问题标题 *</label>
					<input
						id="title"
						type="text"
						bind:value={formData.title}
						placeholder="简要描述您遇到的问题"
						required
					/>
				</div>

				<div class="form-group">
					<label for="description">详细描述</label>
					<textarea
						id="description"
						bind:value={formData.description}
						placeholder="详细描述作物的症状、发病时间、环境等信息"
						rows="4"
					></textarea>
				</div>

				<div class="form-group">
					<label for="cropType">作物类型</label>
					<select id="cropType" bind:value={formData.cropType}>
						{#each cropOptions as option}
							<option value={option.value}>{option.label}</option>
						{/each}
					</select>
				</div>

				<div class="form-group">
					<label for="location">地理位置</label>
					<input
						id="location"
						type="text"
						bind:value={formData.location}
						placeholder="如：湖南省岳阳市XX县XX村"
					/>
					{#if formData.latitude && formData.longitude}
						<p class="location-info">
							📍 已获取位置：{formData.latitude.toFixed(4)}, {formData.longitude.toFixed(4)}
						</p>
					{/if}
				</div>
			</section>

			<div class="form-actions">
				<button type="button" class="btn btn-secondary" on:click={prevStep}>
					← 上一步
				</button>
				<button
					type="button"
					class="btn btn-primary"
					disabled={isSubmitting || !formData.title.trim()}
					on:click={submitDiagnosis}
				>
					{#if isSubmitting}
						<span class="spinner"></span>
						<span>提交中...</span>
					{:else}
						🔍 开始诊断
					{/if}
				</button>
			</div>
		</div>
	{:else if step === 3}
		<div class="step-content result-content">
			{#if diagnosisResult}
				<div class="diagnosis-summary">
					<div class="result-header">
						<span class="result-icon">✅</span>
						<div>
							<h3>AI诊断结果</h3>
							<p class="confidence">
								置信度：{Math.round(diagnosisResult.confidence * 100)}%
							</p>
						</div>
					</div>

					<div class="disease-info">
						<div class="disease-name-row">
							<span class="disease-name">{diagnosisResult.diseaseName}</span>
							<span class="disease-type">{diagnosisResult.diseaseType}</span>
							<span class="severity-badge {severityColors[diagnosisResult.severity]}">
								{diagnosisResult.severity}
							</span>
						</div>
					</div>

					{#if diagnosisResult.symptoms}
						<div class="info-section">
							<h4>📋 症状描述</h4>
							<p>{diagnosisResult.symptoms}</p>
						</div>
					{/if}

					{#if diagnosisResult.causes}
						<div class="info-section">
							<h4>🔍 发病原因</h4>
							<p>{diagnosisResult.causes}</p>
						</div>
					{/if}

					{#if diagnosisResult.recommendedActions}
						<div class="info-section">
							<h4>💡 推荐措施</h4>
							<p>{diagnosisResult.recommendedActions}</p>
						</div>
					{/if}
				</div>

				<div class="next-steps">
					<h4>下一步</h4>
					<ul>
						<li>系统已为您分配附近的农技专家</li>
						<li>您可以与专家在线交流获取更多建议</li>
						<li>专家将为您开具详细的防治方案</li>
					</ul>
				</div>
			{:else}
				<div class="offline-result">
					<span class="result-icon">📥</span>
					<h3>离线模式</h3>
					<p>诊断数据已保存到本地，联网后将自动同步到服务器</p>
				</div>
			{/if}

			<div class="form-actions result-actions">
				<a href="/" class="btn btn-secondary">返回首页</a>
				{#if workOrderId}
					<button type="button" class="btn btn-primary" on:click={viewWorkOrder}>
						查看工单详情 →
					</button>
				{/if}
			</div>
		</div>
	{/if}
</div>

<style>
	.diagnose-container {
		max-width: 700px;
		margin: 0 auto;
	}

	.page-header {
		margin-bottom: 24px;
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

	.steps-indicator {
		display: flex;
		align-items: center;
		justify-content: center;
		margin-bottom: 32px;
		padding: 20px;
		background: white;
		border-radius: 12px;
	}

	.step-item {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 8px;
	}

	.step-number {
		width: 36px;
		height: 36px;
		border-radius: 50%;
		display: flex;
		align-items: center;
		justify-content: center;
		font-weight: 600;
		font-size: 14px;
		background-color: #e5e5e5;
		color: #666;
		transition: all 0.2s;
	}

	.step-item.active .step-number {
		background-color: #2d5a27;
		color: white;
	}

	.step-item.completed .step-number {
		background-color: #4ade80;
		color: white;
	}

	.step-label {
		font-size: 12px;
		color: #666;
	}

	.step-item.active .step-label {
		color: #2d5a27;
		font-weight: 500;
	}

	.step-line {
		flex: 1;
		height: 2px;
		background-color: #e5e5e5;
		margin: 0 16px;
		max-width: 60px;
	}

	.step-line.completed {
		background-color: #4ade80;
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

	.alert-warning {
		background-color: #fef3c7;
		color: #92400e;
		border: 1px solid #fde68a;
	}

	.step-content {
		background: white;
		border-radius: 16px;
		padding: 24px;
		box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
	}

	.step-content h3 {
		margin: 0 0 20px 0;
		font-size: 18px;
	}

	.image-upload-section h3 {
		margin-bottom: 16px;
	}

	.upload-area {
		border: 2px dashed #d1d5db;
		border-radius: 12px;
		padding: 40px;
		text-align: center;
		cursor: pointer;
		transition: all 0.2s;
		margin-bottom: 20px;
	}

	.upload-area:hover {
		border-color: #2d5a27;
		background-color: #f0f7ef;
	}

	.upload-icon {
		font-size: 48px;
		display: block;
		margin-bottom: 12px;
	}

	.upload-area p {
		margin: 0 0 8px 0;
		color: #374151;
	}

	.upload-hint {
		font-size: 12px;
		color: #999;
	}

	.preview-grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(100px, 1fr));
		gap: 12px;
		margin-top: 16px;
	}

	.preview-item {
		position: relative;
		aspect-ratio: 1;
		border-radius: 8px;
		overflow: hidden;
		background: #f5f5f5;
	}

	.preview-item img {
		width: 100%;
		height: 100%;
		object-fit: cover;
	}

	.remove-btn {
		position: absolute;
		top: 4px;
		right: 4px;
		width: 24px;
		height: 24px;
		border-radius: 50%;
		background: rgba(0, 0, 0, 0.6);
		color: white;
		border: none;
		cursor: pointer;
		display: flex;
		align-items: center;
		justify-content: center;
		font-size: 12px;
	}

	.form-group {
		margin-bottom: 20px;
	}

	.form-group label {
		display: block;
		margin-bottom: 8px;
		font-weight: 500;
		color: #374151;
		font-size: 14px;
	}

	.form-group input,
	.form-group textarea,
	.form-group select {
		width: 100%;
		padding: 12px 16px;
		border: 1px solid #d1d5db;
		border-radius: 8px;
		font-size: 14px;
		transition: border-color 0.2s;
		box-sizing: border-box;
	}

	.form-group input:focus,
	.form-group textarea:focus,
	.form-group select:focus {
		outline: none;
		border-color: #2d5a27;
		box-shadow: 0 0 0 3px rgba(74, 124, 68, 0.1);
	}

	.form-group textarea {
		resize: vertical;
		min-height: 100px;
	}

	.location-info {
		margin: 8px 0 0 0;
		font-size: 13px;
		color: #666;
	}

	.form-actions {
		display: flex;
		gap: 12px;
		justify-content: flex-end;
		margin-top: 24px;
		padding-top: 20px;
		border-top: 1px solid #e5e5e5;
	}

	.result-actions {
		justify-content: space-between;
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

	@keyframes spin {
		to { transform: rotate(360deg); }
	}

	.diagnosis-summary {
		margin-bottom: 24px;
	}

	.result-header {
		display: flex;
		align-items: center;
		gap: 16px;
		margin-bottom: 20px;
	}

	.result-icon {
		font-size: 48px;
	}

	.result-header h3 {
		margin: 0 0 4px 0;
		font-size: 18px;
	}

	.confidence {
		margin: 0;
		font-size: 14px;
		color: #666;
	}

	.disease-info {
		background: #f0f7ef;
		padding: 16px;
		border-radius: 12px;
		margin-bottom: 20px;
	}

	.disease-name-row {
		display: flex;
		align-items: center;
		gap: 12px;
		flex-wrap: wrap;
	}

	.disease-name {
		font-size: 20px;
		font-weight: 600;
		color: #2d5a27;
	}

	.disease-type {
		padding: 4px 10px;
		background: white;
		border-radius: 4px;
		font-size: 13px;
		color: #666;
	}

	.severity-badge {
		padding: 4px 12px;
		border-radius: 20px;
		font-size: 13px;
		font-weight: 500;
	}

	.info-section {
		margin-bottom: 20px;
	}

	.info-section h4 {
		margin: 0 0 8px 0;
		font-size: 15px;
		color: #374151;
	}

	.info-section p {
		margin: 0;
		line-height: 1.6;
		color: #555;
	}

	.next-steps {
		background: #fef3c7;
		padding: 16px 20px;
		border-radius: 12px;
		margin-bottom: 24px;
	}

	.next-steps h4 {
		margin: 0 0 12px 0;
		font-size: 15px;
		color: #92400e;
	}

	.next-steps ul {
		margin: 0;
		padding-left: 20px;
	}

	.next-steps li {
		margin-bottom: 6px;
		color: #78350f;
		font-size: 14px;
	}

	.offline-result {
		text-align: center;
		padding: 40px 20px;
	}

	.offline-result .result-icon {
		font-size: 64px;
		display: block;
		margin-bottom: 16px;
	}

	.offline-result h3 {
		margin: 0 0 12px 0;
	}

	.offline-result p {
		margin: 0;
		color: #666;
	}

	@media (max-width: 640px) {
		.steps-indicator {
			padding: 12px;
		}

		.step-line {
			margin: 0 8px;
			max-width: 30px;
		}

		.step-label {
			display: none;
		}

		.form-actions {
			flex-direction: column-reverse;
		}

		.result-actions {
			flex-direction: column;
		}

		.btn {
			width: 100%;
			justify-content: center;
		}
	}
</style>
