import adapter from '@sveltejs/adapter-auto';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
const config = {
	preprocess: vitePreprocess(),

	kit: {
		adapter: adapter(),
		alias: {
			'$lib': './src/lib',
			'$stores': './src/stores',
			'$utils': './src/utils',
			'$components': './src/components'
		}
	}
};

export default config;
