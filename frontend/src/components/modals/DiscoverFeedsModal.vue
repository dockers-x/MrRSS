<script setup>
import { ref, computed, watch, onMounted, onUnmounted } from 'vue';
import { store } from '../../store.js';
import { PhX, PhCheck, PhGlobe, PhRss, PhCircleNotch } from "@phosphor-icons/vue";

const props = defineProps({
    feed: { type: Object, required: true },
    show: { type: Boolean, required: true }
});

const emit = defineEmits(['close']);

const isDiscovering = ref(false);
const discoveredFeeds = ref([]);
const selectedFeeds = ref(new Set());
const errorMessage = ref('');
const progressMessage = ref('');
const progressDetail = ref('');
const progressCounts = ref({ current: 0, total: 0, found: 0 });
const isSubscribing = ref(false);
let eventSource = null;

function getHostname(url) {
    try {
        return new URL(url).hostname;
    } catch {
        return url;
    }
}

async function startDiscovery() {
    console.log('startDiscovery: Beginning SSE discovery process');
    isDiscovering.value = true;
    errorMessage.value = '';
    discoveredFeeds.value = [];
    selectedFeeds.value.clear();
    progressMessage.value = store.i18n.t('fetchingHomepage');
    progressDetail.value = '';
    progressCounts.value = { current: 0, total: 0, found: 0 };

    // Close any existing event source
    if (eventSource) {
        eventSource.close();
        eventSource = null;
    }

    try {
        // Use SSE endpoint for real-time progress
        eventSource = new EventSource(`/api/feeds/discover-sse?feed_id=${props.feed.id}`);

        eventSource.addEventListener('progress', (event) => {
            const progress = JSON.parse(event.data);
            console.log('Progress:', progress);
            
            // Update progress message based on stage
            switch (progress.stage) {
                case 'fetching_homepage':
                    progressMessage.value = store.i18n.t('fetchingHomepage');
                    progressDetail.value = getHostname(progress.detail);
                    break;
                case 'finding_friend_links':
                    progressMessage.value = store.i18n.t('searchingFriendLinks');
                    progressDetail.value = getHostname(progress.detail);
                    break;
                case 'fetching_friend_page':
                    progressMessage.value = store.i18n.t('fetchingFriendPage');
                    progressDetail.value = getHostname(progress.detail);
                    break;
                case 'found_links':
                    progressMessage.value = store.i18n.t('foundPotentialLinks', { count: progress.total });
                    progressDetail.value = '';
                    progressCounts.value.total = progress.total;
                    break;
                case 'checking_rss':
                    progressMessage.value = store.i18n.t('checkingRssFeed');
                    progressDetail.value = getHostname(progress.detail);
                    progressCounts.value.current = progress.current || 0;
                    progressCounts.value.total = progress.total || 0;
                    progressCounts.value.found = progress.found_count || 0;
                    break;
                default:
                    progressMessage.value = progress.message || store.i18n.t('discovering');
                    progressDetail.value = progress.detail ? getHostname(progress.detail) : '';
            }
        });

        eventSource.addEventListener('complete', (event) => {
            const result = JSON.parse(event.data);
            console.log('Discovery complete:', result);
            
            discoveredFeeds.value = result.feeds || [];
            
            if (discoveredFeeds.value.length === 0) {
                errorMessage.value = store.i18n.t('noFriendLinksFound');
            }
            
            isDiscovering.value = false;
            progressMessage.value = '';
            progressDetail.value = '';
            eventSource.close();
            eventSource = null;
        });

        eventSource.addEventListener('error', (event) => {
            // Check if it's a connection error or an error event from server
            if (event.data) {
                const error = JSON.parse(event.data);
                console.error('Discovery error:', error);
                errorMessage.value = store.i18n.t('discoveryFailed') + ': ' + (error.message || 'Unknown error');
            } else {
                console.error('EventSource error:', event);
                // Only show error if we're still discovering (not if SSE just closed)
                if (isDiscovering.value) {
                    errorMessage.value = store.i18n.t('discoveryFailed');
                }
            }
            isDiscovering.value = false;
            progressMessage.value = '';
            progressDetail.value = '';
            eventSource.close();
            eventSource = null;
        });

    } catch (error) {
        console.error('Discovery error:', error);
        errorMessage.value = store.i18n.t('discoveryFailed') + ': ' + error.message;
        isDiscovering.value = false;
        progressMessage.value = '';
        progressDetail.value = '';
    }
}

function toggleFeedSelection(index) {
    if (selectedFeeds.value.has(index)) {
        selectedFeeds.value.delete(index);
    } else {
        selectedFeeds.value.add(index);
    }
}

function selectAll() {
    if (selectedFeeds.value.size === discoveredFeeds.value.length) {
        selectedFeeds.value.clear();
    } else {
        discoveredFeeds.value.forEach((_, index) => selectedFeeds.value.add(index));
    }
}

const hasSelection = computed(() => selectedFeeds.value.size > 0);
const allSelected = computed(() => discoveredFeeds.value.length > 0 && selectedFeeds.value.size === discoveredFeeds.value.length);

async function subscribeSelected() {
    if (!hasSelection.value) return;

    isSubscribing.value = true;
    const subscribePromises = [];
    
    for (const index of selectedFeeds.value) {
        const feed = discoveredFeeds.value[index];
        const promise = fetch('/api/feeds/add', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                url: feed.rss_feed,
                category: props.feed.category || '',
                title: feed.name
            })
        });
        subscribePromises.push(promise);
    }

    try {
        const results = await Promise.allSettled(subscribePromises);
        const successful = results.filter(r => r.status === 'fulfilled').length;
        const failed = results.filter(r => r.status === 'rejected').length;
        
        await store.fetchFeeds();
        
        if (failed === 0) {
            window.showToast(store.i18n.t('feedsSubscribedSuccess', { count: successful }), 'success');
        } else {
            window.showToast(store.i18n.t('feedsSubscribedPartial', { successful, failed }), 'warning');
        }
        emit('close');
    } catch (error) {
        console.error('Subscription error:', error);
        window.showToast(store.i18n.t('errorSubscribingFeeds'), 'error');
    } finally {
        isSubscribing.value = false;
    }
}

function close() {
    // Close SSE connection if active
    if (eventSource) {
        eventSource.close();
        eventSource = null;
    }
    emit('close');
}

// Auto-start discovery when component is mounted
onMounted(() => {
    console.log('DiscoverFeedsModal: Component mounted, show =', props.show);
    if (props.show) {
        console.log('DiscoverFeedsModal: Auto-starting discovery on mount');
        startDiscovery();
    }
});

// Watch for modal opening and trigger discovery (for when modal is reused)
watch(() => props.show, (newShow, oldShow) => {
    console.log('DiscoverFeedsModal: show changed from', oldShow, 'to', newShow);
    if (newShow && !oldShow) {
        console.log('DiscoverFeedsModal: Starting discovery from watch');
        startDiscovery();
    }
});

// Cleanup on unmount
onUnmounted(() => {
    if (eventSource) {
        eventSource.close();
        eventSource = null;
    }
});
</script>

<template>
    <div v-if="show" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm p-4" @click.self="close">
        <div class="bg-bg-primary w-full max-w-4xl max-h-[90vh] rounded-2xl shadow-2xl border border-border flex flex-col">
            <!-- Header -->
            <div class="flex justify-between items-center p-6 border-b border-border bg-gradient-to-r from-accent/5 to-transparent">
                <div>
                    <h2 class="text-xl font-bold text-text-primary">{{ store.i18n.t('discoverFeeds') }}</h2>
                    <p class="text-sm text-text-secondary mt-1">{{ store.i18n.t('fromFeed') }}: {{ feed.title }}</p>
                </div>
                <button @click="close" class="p-2 hover:bg-bg-tertiary rounded-lg transition-colors">
                    <PhX :size="24" class="text-text-secondary" />
                </button>
            </div>

            <!-- Content -->
            <div class="flex-1 overflow-y-auto p-6">
                <!-- Loading State -->
                <div v-if="isDiscovering" class="flex flex-col items-center justify-center py-12">
                    <PhCircleNotch :size="48" class="text-accent animate-spin mb-4" />
                    <p class="text-text-primary font-medium mb-2">{{ store.i18n.t('discovering') }}</p>
                    <p v-if="progressMessage" class="text-sm text-text-secondary">{{ progressMessage }}</p>
                    <p v-if="progressDetail" class="text-xs text-text-tertiary mt-1 font-mono">{{ progressDetail }}</p>
                    <div v-if="progressCounts.total > 0" class="mt-3 text-xs text-text-tertiary">
                        <span>{{ progressCounts.current }}/{{ progressCounts.total }}</span>
                        <span v-if="progressCounts.found > 0" class="ml-2">• {{ store.i18n.t('foundSoFar', { count: progressCounts.found }) }}</span>
                    </div>
                </div>

                <!-- Error State -->
                <div v-else-if="errorMessage" class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-4 text-red-600 dark:text-red-400">
                    {{ errorMessage }}
                </div>

                <!-- Results -->
                <div v-else-if="discoveredFeeds.length > 0">
                    <div class="mb-4 flex items-center justify-between bg-bg-secondary rounded-lg p-3">
                        <p class="text-sm font-medium text-text-primary">
                            {{ store.i18n.t('foundFeeds', { count: discoveredFeeds.length }) }}
                        </p>
                        <button @click="selectAll" class="text-sm text-accent hover:text-accent-hover font-medium px-3 py-1 rounded hover:bg-accent/10 transition-colors">
                            {{ allSelected ? store.i18n.t('deselectAll') : store.i18n.t('selectAll') }}
                        </button>
                    </div>

                    <div class="space-y-3">
                        <div v-for="(feed, index) in discoveredFeeds" :key="index" 
                             @click="toggleFeedSelection(index)"
                             :class="[
                                 'border rounded-xl p-4 cursor-pointer transition-all duration-200',
                                 selectedFeeds.has(index) 
                                     ? 'bg-accent/10 border-accent ring-2 ring-accent/20 shadow-md' 
                                     : 'bg-bg-secondary hover:bg-bg-tertiary border-border hover:shadow-sm'
                             ]">
                            <div class="flex items-start gap-4">
                                <!-- Checkbox -->
                                <div class="mt-1 shrink-0">
                                    <div :class="[
                                        'w-5 h-5 rounded border-2 flex items-center justify-center transition-all',
                                        selectedFeeds.has(index) 
                                            ? 'bg-accent border-accent scale-110' 
                                            : 'border-border bg-bg-primary'
                                    ]">
                                        <PhCheck v-if="selectedFeeds.has(index)" :size="14" weight="bold" class="text-white" />
                                    </div>
                                </div>

                                <!-- Feed Info -->
                                <div class="flex-1 min-w-0">
                                    <div class="flex items-start gap-3 mb-3">
                                        <div class="shrink-0 w-10 h-10 rounded-lg overflow-hidden bg-bg-primary border border-border flex items-center justify-center">
                                            <img :src="feed.icon_url" class="w-full h-full object-cover" :alt="feed.name" @error="$event.target.style.display='none'">
                                        </div>
                                        <div class="flex-1 min-w-0">
                                            <h3 class="font-semibold text-text-primary truncate text-base">{{ feed.name }}</h3>
                                            <a :href="feed.homepage" 
                                               target="_blank" 
                                               @click.stop
                                               class="text-xs text-accent hover:text-accent-hover flex items-center gap-1 mt-1 hover:underline">
                                                <PhGlobe :size="14" />
                                                <span class="truncate">{{ feed.homepage }}</span>
                                            </a>
                                        </div>
                                    </div>

                                    <!-- Recent Articles -->
                                    <div v-if="feed.recent_articles && feed.recent_articles.length > 0" class="mt-3">
                                        <p class="text-xs font-semibold text-text-secondary mb-2 flex items-center gap-1">
                                            <PhRss :size="12" />
                                            {{ store.i18n.t('recentArticles') }}
                                        </p>
                                        <div class="space-y-1.5">
                                            <div v-for="(article, aIndex) in feed.recent_articles" :key="aIndex" 
                                                 class="flex flex-col gap-0.5 py-1.5 border-l-2 border-border pl-2">
                                                <span class="text-sm text-text-primary line-clamp-2 leading-snug">
                                                    {{ article.title || article }}
                                                </span>
                                                <span v-if="article.date" class="text-xs text-text-tertiary">
                                                    {{ article.date }}
                                                </span>
                                            </div>
                                        </div>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>

                <!-- Initial State (should not be visible as discovery auto-starts) -->
                <div v-else class="text-center py-16">
                    <PhCircleNotch :size="64" class="text-accent mx-auto mb-4 animate-spin" />
                    <p class="text-text-secondary text-lg">{{ store.i18n.t('preparing') }}...</p>
                </div>
            </div>

            <!-- Footer -->
            <div class="flex justify-between items-center p-6 border-t border-border bg-bg-secondary/50">
                <button @click="close" class="btn-secondary" :disabled="isSubscribing">
                    {{ store.i18n.t('cancel') }}
                </button>
                <button @click="subscribeSelected" 
                        :disabled="!hasSelection || isSubscribing" 
                        :class="['btn-primary flex items-center gap-2', (!hasSelection || isSubscribing) && 'opacity-50 cursor-not-allowed']">
                    <PhCircleNotch v-if="isSubscribing" :size="16" class="animate-spin" />
                    {{ isSubscribing ? store.i18n.t('subscribing') : store.i18n.t('subscribeSelected') }}
                    <span v-if="hasSelection && !isSubscribing" class="bg-white/20 px-2 py-0.5 rounded-full text-sm">({{ selectedFeeds.size }})</span>
                </button>
            </div>
        </div>
    </div>
</template>

<style scoped>
.btn-primary {
    @apply px-6 py-2.5 bg-accent text-white rounded-lg hover:bg-accent-hover transition-all font-medium shadow-sm hover:shadow-md;
}

.btn-secondary {
    @apply px-6 py-2.5 bg-bg-tertiary text-text-primary rounded-lg hover:opacity-80 transition-all font-medium;
}
</style>
