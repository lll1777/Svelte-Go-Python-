import { writable, derived } from 'svelte/store';
import { apiClient } from '$lib/api/client';
import { offlineStorage } from '$lib/utils/offlineStorage';

function createAuthStore() {
    const initialUser = null;
    const initialToken = localStorage.getItem('auth_token');

    const { subscribe, set, update } = writable({
        user: initialUser,
        token: initialToken,
        isAuthenticated: !!initialToken,
        isLoading: false,
        error: null
    });

    return {
        subscribe,

        async login(username, password) {
            update(state => ({ ...state, isLoading: true, error: null }));

            try {
                const response = await apiClient.post('/auth/login', { username, password });
                
                const { token, user } = response;
                apiClient.setToken(token);

                await offlineStorage.setUserData('user', user);

                set({
                    user,
                    token,
                    isAuthenticated: true,
                    isLoading: false,
                    error: null
                });

                return { success: true, user };
            } catch (error) {
                update(state => ({
                    ...state,
                    isLoading: false,
                    error: error.message
                }));
                return { success: false, error: error.message };
            }
        },

        async register(userData) {
            update(state => ({ ...state, isLoading: true, error: null }));

            try {
                const response = await apiClient.post('/auth/register', userData);
                
                update(state => ({
                    ...state,
                    isLoading: false,
                    error: null
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

        async logout() {
            apiClient.clearToken();
            await offlineStorage.setUserData('user', null);
            
            set({
                user: null,
                token: null,
                isAuthenticated: false,
                isLoading: false,
                error: null
            });
        },

        async fetchProfile() {
            try {
                const user = await apiClient.get('/users/profile');
                
                update(state => ({
                    ...state,
                    user,
                    isAuthenticated: true
                }));

                await offlineStorage.setUserData('user', user);
                return user;
            } catch (error) {
                console.error('Failed to fetch profile:', error);
                
                const cachedUser = await offlineStorage.getUserData('user');
                if (cachedUser) {
                    update(state => ({
                        ...state,
                        user: cachedUser,
                        isAuthenticated: true
                    }));
                    return cachedUser;
                }

                return null;
            }
        },

        async updateProfile(profileData, role) {
            try {
                const endpoint = role === 'farmer' 
                    ? '/users/profile/farmer' 
                    : '/users/profile/expert';
                
                const response = await apiClient.put(endpoint, profileData);
                
                update(state => ({
                    ...state,
                    user: { ...state.user, ...profileData }
                }));

                return { success: true, data: response };
            } catch (error) {
                return { success: false, error: error.message };
            }
        }
    };
}

export const auth = createAuthStore();

export const isAuthenticated = derived(auth, $auth => $auth.isAuthenticated);
export const currentUser = derived(auth, $auth => $auth.user);
export const userRole = derived(auth, $auth => $auth.user?.role);
