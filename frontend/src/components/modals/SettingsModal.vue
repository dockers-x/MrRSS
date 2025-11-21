<script setup>
import { store } from '../../store.js';
import { ref, onMounted, computed } from 'vue';

const emit = defineEmits(['close']);
const activeTab = ref('general');
const selectedFeeds = ref([]);

const settings = ref({
    update_interval: 10,
    translation_enabled: false,
    target_language: 'zh',
    translation_provider: 'google',
    deepl_api_key: '',
    auto_cleanup_enabled: false,
    language: store.i18n.locale.value,
    theme: 'auto'
});

const updateInfo = ref(null);
const checkingUpdates = ref(false);
const appVersion = ref('1.1.2');

onMounted(async () => {
    // Fetch current version from API
    try {
        const versionRes = await fetch('/api/version');
        if (versionRes.ok) {
            const versionData = await versionRes.json();
            appVersion.value = versionData.version;
        }
    } catch (e) {
        console.error('Error fetching version:', e);
    }

    // Fetch settings
    try {
        const res = await fetch('/api/settings');
        const data = await res.json();
        settings.value = {
            update_interval: data.update_interval || 10,
            translation_enabled: data.translation_enabled === 'true',
            target_language: data.target_language || 'zh',
            translation_provider: data.translation_provider || 'google',
            deepl_api_key: data.deepl_api_key || '',
            auto_cleanup_enabled: data.auto_cleanup_enabled === 'true',
            language: data.language || store.i18n.locale.value,
            theme: data.theme || 'auto'
        };
        // Apply the saved language
        if (data.language) {
            store.i18n.setLocale(data.language);
        }
        // Apply the saved theme
        if (data.theme) {
            store.setTheme(data.theme);
        }
    } catch (e) {
        console.error(e);
    }
});

async function saveSettings() {
    try {
        await fetch('/api/settings', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                update_interval: settings.value.update_interval.toString(),
                translation_enabled: settings.value.translation_enabled.toString(),
                target_language: settings.value.target_language,
                translation_provider: settings.value.translation_provider,
                deepl_api_key: settings.value.deepl_api_key,
                auto_cleanup_enabled: settings.value.auto_cleanup_enabled.toString(),
                language: settings.value.language,
                theme: settings.value.theme
            })
        });
        store.i18n.setLocale(settings.value.language);
        store.setTheme(settings.value.theme);
        store.startAutoRefresh(settings.value.update_interval);
        emit('close');
    } catch (e) {
        window.showToast(store.i18n.t('errorSavingSettings'), 'error');
    }
}

function showAddFeedModal() {
    window.dispatchEvent(new CustomEvent('show-add-feed'));
}

function importOPML(event) {
    const file = event.target.files[0];
    if (!file) return;
    
    const reader = new FileReader();
    reader.onload = (e) => {
        const content = e.target.result;
        fetch('/api/opml/import', {
            method: 'POST',
            headers: {
                'Content-Type': 'text/xml'
            },
            body: content
        }).then(async res => {
            if (res.ok) {
                window.showToast('OPML Imported. Feeds will appear shortly.', 'success');
                store.fetchFeeds();
            } else {
                const text = await res.text();
                window.showToast('Import failed: ' + text, 'error');
            }
        });
    };
    reader.readAsText(file);
}

function exportOPML() {
    window.location.href = '/api/opml/export';
}

async function deleteFeed(id) {
    const confirmed = await window.showConfirm({
        title: 'Delete Feed',
        message: 'Are you sure you want to delete this feed?',
        confirmText: 'Delete',
        cancelText: 'Cancel',
        isDanger: true
    });
    if (!confirmed) return;
    
    await fetch(`/api/feeds/delete?id=${id}`, { method: 'POST' });
    store.fetchFeeds();
    window.showToast('Feed deleted successfully', 'success');
}

const isAllSelected = computed(() => {
    return store.feeds && store.feeds.length > 0 && selectedFeeds.value.length === store.feeds.length;
});

function toggleSelectAll(e) {
    if (!store.feeds) return;
    if (e.target.checked) {
        selectedFeeds.value = store.feeds.map(f => f.id);
    } else {
        selectedFeeds.value = [];
    }
}

async function batchDelete() {
    if (selectedFeeds.value.length === 0) return;
    
    const confirmed = await window.showConfirm({
        title: 'Delete Multiple Feeds',
        message: `Are you sure you want to delete ${selectedFeeds.value.length} feeds?`,
        confirmText: 'Delete',
        cancelText: 'Cancel',
        isDanger: true
    });
    if (!confirmed) return;

    const promises = selectedFeeds.value.map(id => fetch(`/api/feeds/delete?id=${id}`, { method: 'POST' }));
    await Promise.all(promises);
    selectedFeeds.value = [];
    store.fetchFeeds();
    window.showToast('Feeds deleted successfully', 'success');
}

async function batchMove() {
    if (selectedFeeds.value.length === 0) return;
    if (!store.feeds) return;
    
    // TODO: Replace with custom input dialog
    const newCategory = prompt('Enter new category name:');
    if (newCategory === null) return;

    const promises = selectedFeeds.value.map(id => {
        const feed = store.feeds.find(f => f.id === id);
        if (!feed) return Promise.resolve();
        return fetch('/api/feeds/update', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ 
                id: feed.id, 
                title: feed.title, 
                url: feed.url, 
                category: newCategory 
            })
        });
    });

    await Promise.all(promises);
    selectedFeeds.value = [];
    store.fetchFeeds();
    window.showToast('Feeds moved successfully', 'success');
}

function editFeed(feed) {
    window.dispatchEvent(new CustomEvent('show-edit-feed', { detail: feed }));
}

async function cleanupDatabase() {
    const confirmed = await window.showConfirm({
        title: 'Clean Database',
        message: 'This will delete all articles except read and favorited ones. Continue?',
        confirmText: 'Clean',
        cancelText: 'Cancel',
        isDanger: true
    });
    if (!confirmed) return;
    
    try {
        const res = await fetch('/api/articles/cleanup', { method: 'POST' });
        if (res.ok) {
            const result = await res.json();
            window.showToast(`Database cleaned up successfully. ${result.deleted} articles deleted.`, 'success');
            store.fetchArticles();
        } else {
            window.showToast('Error cleaning up database', 'error');
        }
    } catch (e) {
        console.error(e);
        window.showToast('Error cleaning up database', 'error');
    }
}

async function checkForUpdates() {
    checkingUpdates.value = true;
    updateInfo.value = null;
    
    try {
        const res = await fetch('/api/check-updates');
        if (res.ok) {
            const data = await res.json();
            updateInfo.value = data;
            
            if (data.error) {
                window.showToast(store.i18n.t('errorCheckingUpdates'), 'error');
            } else if (data.has_update) {
                window.showToast(store.i18n.t('updateAvailable'), 'info');
            } else {
                window.showToast(store.i18n.t('upToDate'), 'success');
            }
        } else {
            window.showToast(store.i18n.t('errorCheckingUpdates'), 'error');
        }
    } catch (e) {
        console.error(e);
        window.showToast(store.i18n.t('errorCheckingUpdates'), 'error');
    } finally {
        checkingUpdates.value = false;
    }
}

</script>

<template>
    <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm p-4">
        <div class="bg-bg-primary w-full max-w-4xl h-[700px] flex flex-col rounded-2xl shadow-2xl border border-border overflow-hidden animate-fade-in">
            <div class="p-5 border-b border-border flex justify-between items-center shrink-0">
                <h3 class="text-lg font-semibold m-0 flex items-center gap-2">
                    <i class="ph ph-gear text-xl"></i>
                    {{ store.i18n.t('settingsTitle') }}
                </h3>
                <span @click="emit('close')" class="text-2xl cursor-pointer text-text-secondary hover:text-text-primary">&times;</span>
            </div>
            
            <div class="flex border-b border-border bg-bg-secondary shrink-0 overflow-x-auto">
                <button @click="activeTab = 'general'" :class="['tab-btn', activeTab === 'general' ? 'active' : '']">{{ store.i18n.t('general') }}</button>
                <button @click="activeTab = 'feeds'" :class="['tab-btn', activeTab === 'feeds' ? 'active' : '']">{{ store.i18n.t('feeds') }}</button>
                <button @click="activeTab = 'about'" :class="['tab-btn', activeTab === 'about' ? 'active' : '']">{{ store.i18n.t('about') }}</button>
            </div>

            <div class="flex-1 overflow-y-auto p-6">
                <div v-if="activeTab === 'general'" class="space-y-6">
                    <div class="setting-group">
                        <label class="block font-semibold mb-3 text-text-secondary uppercase text-xs tracking-wider flex items-center gap-2">
                            <i class="ph ph-palette text-base"></i>
                            {{ store.i18n.t('appearance') }}
                        </label>
                        <div class="setting-item">
                            <div class="flex-1 flex items-start gap-3">
                                <i class="ph ph-moon text-xl text-text-secondary mt-0.5"></i>
                                <div class="flex-1">
                                    <div class="font-medium mb-1">{{ store.i18n.t('theme') }}</div>
                                    <div class="text-xs text-text-secondary">{{ store.i18n.t('themeDesc') }}</div>
                                </div>
                            </div>
                            <select v-model="settings.theme" class="input-field w-40">
                                <option value="light">{{ store.i18n.t('light') }}</option>
                                <option value="dark">{{ store.i18n.t('dark') }}</option>
                                <option value="auto">{{ store.i18n.t('auto') }}</option>
                            </select>
                        </div>
                        <div class="setting-item mt-3">
                            <div class="flex-1 flex items-start gap-3">
                                <i class="ph ph-translate text-xl text-text-secondary mt-0.5"></i>
                                <div class="flex-1">
                                    <div class="font-medium mb-1">{{ store.i18n.t('language') }}</div>
                                    <div class="text-xs text-text-secondary">{{ store.i18n.t('languageDesc') }}</div>
                                </div>
                            </div>
                            <select v-model="settings.language" class="input-field w-32">
                                <option value="en">{{ store.i18n.t('english') }}</option>
                                <option value="zh">{{ store.i18n.t('chinese') }}</option>
                            </select>
                        </div>
                    </div>

                    <div class="setting-group">
                        <label class="block font-semibold mb-3 text-text-secondary uppercase text-xs tracking-wider flex items-center gap-2">
                            <i class="ph ph-arrow-clockwise text-base"></i>
                            {{ store.i18n.t('updates') }}
                        </label>
                        <div class="setting-item">
                            <div class="flex-1 flex items-start gap-3">
                                <i class="ph ph-clock text-xl text-text-secondary mt-0.5"></i>
                                <div class="flex-1">
                                    <div class="font-medium mb-1">{{ store.i18n.t('autoUpdateInterval') }}</div>
                                    <div class="text-xs text-text-secondary">{{ store.i18n.t('autoUpdateIntervalDesc') }}</div>
                                </div>
                            </div>
                            <input type="number" v-model="settings.update_interval" min="1" class="input-field w-20 text-center">
                        </div>
                    </div>

                    <div class="setting-group">
                        <label class="block font-semibold mb-3 text-text-secondary uppercase text-xs tracking-wider flex items-center gap-2">
                            <i class="ph ph-database text-base"></i>
                            {{ store.i18n.t('database') }}
                        </label>
                        <div class="setting-item">
                            <div class="flex-1 flex items-start gap-3">
                                <i class="ph ph-broom text-xl text-text-secondary mt-0.5"></i>
                                <div class="flex-1">
                                    <div class="font-medium mb-1">{{ store.i18n.t('autoCleanup') }}</div>
                                    <div class="text-xs text-text-secondary">{{ store.i18n.t('autoCleanupDesc') }}</div>
                                </div>
                            </div>
                            <input type="checkbox" v-model="settings.auto_cleanup_enabled" class="toggle">
                        </div>
                    </div>

                    <div class="setting-group">
                        <label class="block font-semibold mb-3 text-text-secondary uppercase text-xs tracking-wider flex items-center gap-2">
                            <i class="ph ph-globe text-base"></i>
                            {{ store.i18n.t('translation') }}
                        </label>
                        <div class="setting-item mb-4">
                            <div class="flex-1 flex items-start gap-3">
                                <i class="ph ph-article text-xl text-text-secondary mt-0.5"></i>
                                <div class="flex-1">
                                    <div class="font-medium mb-1">{{ store.i18n.t('enableTranslation') }}</div>
                                    <div class="text-xs text-text-secondary">{{ store.i18n.t('enableTranslationDesc') }}</div>
                                </div>
                            </div>
                            <input type="checkbox" v-model="settings.translation_enabled" class="toggle">
                        </div>
                        
                        <div v-if="settings.translation_enabled" class="ml-4 space-y-3 border-l-2 border-border pl-4">
                            <div>
                                <label class="block text-sm font-medium mb-1">{{ store.i18n.t('translationProvider') }}</label>
                                <select v-model="settings.translation_provider" class="input-field w-full">
                                    <option value="google">Google Translate (Free)</option>
                                    <option value="deepl">DeepL API</option>
                                </select>
                            </div>
                            <div v-if="settings.translation_provider === 'deepl'">
                                <label class="block text-sm font-medium mb-1">{{ store.i18n.t('deeplApiKey') }}</label>
                                <input type="password" v-model="settings.deepl_api_key" :placeholder="store.i18n.t('deeplApiKeyPlaceholder')" class="input-field w-full">
                            </div>
                            <div>
                                <label class="block text-sm font-medium mb-1">{{ store.i18n.t('targetLanguage') }}</label>
                                <select v-model="settings.target_language" class="input-field w-full">
                                    <option value="en">{{ store.i18n.t('english') }}</option>
                                    <option value="es">{{ store.i18n.t('spanish') }}</option>
                                    <option value="fr">{{ store.i18n.t('french') }}</option>
                                    <option value="de">{{ store.i18n.t('german') }}</option>
                                    <option value="zh">{{ store.i18n.t('chinese') }}</option>
                                    <option value="ja">{{ store.i18n.t('japanese') }}</option>
                                </select>
                            </div>
                        </div>
                    </div>
                </div>

                <div v-if="activeTab === 'feeds'" class="space-y-6">
                    <div class="setting-group">
                        <label class="block font-semibold mb-3 text-text-secondary uppercase text-xs tracking-wider flex items-center gap-2">
                            <i class="ph ph-hard-drives text-base"></i>
                            {{ store.i18n.t('dataManagement') }}
                        </label>
                        <div class="flex gap-3 mb-3">
                            <button @click="$refs.opmlInput.click()" class="btn-secondary flex-1 justify-center">
                                <i class="ph ph-upload"></i> {{ store.i18n.t('importOPML') }}
                            </button>
                            <input type="file" ref="opmlInput" class="hidden" @change="importOPML">
                            <button @click="exportOPML" class="btn-secondary flex-1 justify-center">
                                <i class="ph ph-download"></i> {{ store.i18n.t('exportOPML') }}
                            </button>
                        </div>
                        <div class="flex gap-3">
                            <button @click="cleanupDatabase" class="btn-danger flex-1 justify-center">
                                <i class="ph ph-broom"></i> {{ store.i18n.t('cleanDatabase') }}
                            </button>
                        </div>
                        <p class="text-xs text-text-secondary mt-2">
                            {{ store.i18n.t('cleanDatabaseDesc') }}
                        </p>
                    </div>
                    
                    <div class="setting-group">
                        <label class="block font-semibold mb-3 text-text-secondary uppercase text-xs tracking-wider flex items-center gap-2">
                            <i class="ph ph-rss text-base"></i>
                            {{ store.i18n.t('manageFeeds') }}
                        </label>
                        
                        <div class="flex flex-wrap gap-2 mb-2">
                            <button @click="showAddFeedModal" class="btn-secondary text-sm py-1.5 px-3">
                                <i class="ph ph-plus"></i> {{ store.i18n.t('addFeed') }}
                            </button>
                            <button @click="batchDelete" class="btn-danger text-sm py-1.5 px-3" :disabled="selectedFeeds.length === 0">
                                <i class="ph ph-trash"></i> {{ store.i18n.t('deleteSelected') }}
                            </button>
                            <button @click="batchMove" class="btn-secondary text-sm py-1.5 px-3" :disabled="selectedFeeds.length === 0">
                                <i class="ph ph-folder"></i> {{ store.i18n.t('moveSelected') }}
                            </button>
                            <div class="flex-1"></div>
                            <label class="flex items-center gap-2 text-sm cursor-pointer select-none">
                                <input type="checkbox" :checked="isAllSelected" @change="toggleSelectAll" class="w-4 h-4 rounded border-border text-accent focus:ring-2 focus:ring-accent cursor-pointer">
                                {{ store.i18n.t('selectAll') }}
                            </label>
                        </div>

                        <div class="border border-border rounded-lg bg-bg-secondary overflow-y-auto max-h-96">
                            <div v-for="feed in store.feeds" :key="feed.id" class="flex items-center p-2 border-b border-border last:border-0 bg-bg-primary hover:bg-bg-secondary gap-2">
                                <input type="checkbox" :value="feed.id" v-model="selectedFeeds" class="w-4 h-4 shrink-0 rounded border-border text-accent focus:ring-2 focus:ring-accent cursor-pointer">
                                <div class="truncate flex-1 min-w-0">
                                    <div class="font-medium truncate text-sm">{{ feed.title }}</div>
                                    <div class="text-xs text-text-secondary truncate">{{ feed.url }}</div>
                                </div>
                                <div class="flex gap-1 shrink-0">
                                    <button @click="editFeed(feed)" class="text-accent hover:bg-bg-tertiary p-1 rounded text-sm" :title="store.i18n.t('edit')"><i class="ph ph-pencil"></i></button>
                                    <button @click="deleteFeed(feed.id)" class="text-red-500 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 p-1 rounded text-sm" :title="store.i18n.t('delete')"><i class="ph ph-trash"></i></button>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>

                <div v-if="activeTab === 'about'" class="text-center py-10">
                    <img src="/assets/logo.svg" alt="Logo" class="h-16 w-auto mb-4 mx-auto">
                    <h3 class="text-xl font-bold mb-2">{{ store.i18n.t('appName') }}</h3>
                    <p class="text-text-secondary">{{ store.i18n.t('aboutApp') }}</p>
                    <p class="text-text-secondary text-sm mt-2">{{ store.i18n.t('version') }} {{ appVersion }}</p>
                    
                    <div class="mt-6 mb-6 flex justify-center">
                        <button @click="checkForUpdates" :disabled="checkingUpdates" class="btn-secondary justify-center">
                            <i class="ph ph-arrows-clockwise" :class="{'animate-spin': checkingUpdates}"></i>
                            {{ checkingUpdates ? store.i18n.t('checking') : store.i18n.t('checkForUpdates') }}
                        </button>
                    </div>

                    <div v-if="updateInfo && !updateInfo.error" class="mt-4 mx-auto max-w-md text-left bg-bg-secondary p-4 rounded-lg border border-border">
                        <div class="flex items-start gap-3">
                            <i v-if="updateInfo.has_update" class="ph ph-arrow-circle-up text-2xl text-green-500 mt-0.5"></i>
                            <i v-else class="ph ph-check-circle text-2xl text-accent mt-0.5"></i>
                            <div class="flex-1">
                                <h4 class="font-semibold mb-1">
                                    {{ updateInfo.has_update ? store.i18n.t('updateAvailable') : store.i18n.t('upToDate') }}
                                </h4>
                                <div class="text-sm text-text-secondary space-y-1">
                                    <div>{{ store.i18n.t('currentVersion') }}: {{ updateInfo.current_version }}</div>
                                    <div v-if="updateInfo.has_update">{{ store.i18n.t('latestVersion') }}: {{ updateInfo.latest_version }}</div>
                                </div>
                            </div>
                        </div>
                    </div>

                    <div class="mt-6">
                        <a href="https://github.com/WCY-dt/MrRSS" target="_blank" class="inline-flex items-center gap-2 text-accent hover:text-accent-hover transition-colors text-sm font-medium">
                            <i class="ph ph-github-logo text-xl"></i>
                            {{ store.i18n.t('viewOnGitHub') }}
                        </a>
                    </div>
                </div>
            </div>

            <div class="p-3 border-t border-border bg-bg-secondary text-right shrink-0">
                <button @click="saveSettings" class="btn-primary">{{ store.i18n.t('saveSettings') }}</button>
            </div>
        </div>
    </div>
</template>

<style scoped>
.tab-btn {
    @apply px-5 py-3 bg-transparent border-b-2 border-transparent text-text-secondary font-semibold cursor-pointer hover:text-text-primary transition-colors;
}
.tab-btn.active {
    @apply text-accent border-accent;
}
.input-field {
    @apply p-2.5 border border-border rounded-md bg-bg-secondary text-text-primary text-sm focus:border-accent focus:outline-none transition-colors;
}
.btn-primary {
    @apply bg-accent text-white border-none px-5 py-2.5 rounded-lg cursor-pointer font-semibold hover:bg-accent-hover transition-colors;
}
.btn-secondary {
    @apply bg-transparent border border-border text-text-primary px-4 py-2 rounded-md cursor-pointer flex items-center gap-2 font-medium hover:bg-bg-tertiary transition-colors;
}
.btn-secondary:disabled {
    @apply opacity-50 cursor-not-allowed;
}
.btn-danger {
    @apply bg-transparent border border-red-300 text-red-600 px-4 py-2 rounded-md cursor-pointer flex items-center gap-2 font-semibold hover:bg-red-50 dark:hover:bg-red-900/20 dark:border-red-400 dark:text-red-400 transition-colors;
}
.btn-danger:disabled {
    @apply opacity-50 cursor-not-allowed;
}
.toggle {
    @apply w-10 h-5 appearance-none bg-bg-tertiary rounded-full relative cursor-pointer border border-border transition-colors checked:bg-accent checked:border-accent;
}
.toggle::after {
    content: '';
    @apply absolute top-0.5 left-0.5 w-3.5 h-3.5 bg-white rounded-full shadow-sm transition-transform;
}
.toggle:checked::after {
    transform: translateX(20px);
}
.animate-fade-in {
    animation: modalFadeIn 0.3s cubic-bezier(0.16, 1, 0.3, 1);
}
@keyframes modalFadeIn {
    from { transform: translateY(-20px); opacity: 0; }
    to { transform: translateY(0); opacity: 1; }
}
.setting-item {
    @apply flex items-start justify-between gap-4 p-3 rounded-lg bg-bg-secondary border border-border;
}
.animate-spin {
    animation: spin 1s linear infinite;
}
@keyframes spin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
}
</style>
