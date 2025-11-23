<script setup>
import { store } from '../store.js';
import { ref, computed, onMounted, onBeforeUnmount } from 'vue';
import { BrowserOpenURL } from '../wailsjs/wailsjs/runtime/runtime.js';

const listRef = ref(null);
const translationSettings = ref({
    enabled: false,
    targetLang: 'en'
});
const translatingArticles = ref(new Set());

const props = defineProps(['isSidebarOpen']);
const emit = defineEmits(['toggleSidebar']);

// Load translation settings
onMounted(async () => {
    try {
        const res = await fetch('/api/settings');
        const data = await res.json();
        translationSettings.value = {
            enabled: data.translation_enabled === 'true',
            targetLang: data.target_language || 'en'
        };
        
        // Set up intersection observer for auto-translation
        if (translationSettings.value.enabled) {
            setupIntersectionObserver();
        }
    } catch (e) {
        console.error('Error loading translation settings:', e);
    }
});

// Intersection Observer for auto-translation
let observer = null;

function setupIntersectionObserver() {
    observer = new IntersectionObserver((entries) => {
        entries.forEach(entry => {
            if (entry.isIntersecting) {
                const articleId = parseInt(entry.target.dataset.articleId);
                const article = store.articles.find(a => a.id === articleId);
                
                // Only translate if article exists, has no translation, and is not already being translated
                if (article && !article.translated_title && !translatingArticles.value.has(articleId)) {
                    translateArticle(article);
                }
            }
        });
    }, {
        root: listRef.value,
        rootMargin: '100px',
        threshold: 0.1
    });
}

async function translateArticle(article) {
    if (translatingArticles.value.has(article.id)) return;
    
    translatingArticles.value.add(article.id);
    
    try {
        const res = await fetch('/api/articles/translate', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                article_id: article.id,
                title: article.title,
                target_language: translationSettings.value.targetLang
            })
        });
        
        if (res.ok) {
            const data = await res.json();
            // Update the article in the store
            article.translated_title = data.translated_title;
        }
    } catch (e) {
        console.error('Error translating article:', e);
    } finally {
        translatingArticles.value.delete(article.id);
    }
}

function observeArticle(el) {
    if (el && observer && translationSettings.value.enabled) {
        observer.observe(el);
    }
}

onBeforeUnmount(() => {
    if (observer) {
        observer.disconnect();
    }
});

function handleScroll(e) {
    const { scrollTop, clientHeight, scrollHeight } = e.target;
    if (scrollTop + clientHeight >= scrollHeight - 200) {
        store.loadMore();
    }
}

function selectArticle(article) {
    store.currentArticleId = article.id;
    if (!article.is_read) {
        article.is_read = true;
        fetch(`/api/articles/read?id=${article.id}&read=true`, { method: 'POST' });
    }
}

function formatDate(dateStr) {
    return new Date(dateStr).toLocaleDateString();
}

// Search filtering
const searchQuery = ref('');
const filteredArticles = computed(() => {
    if (!searchQuery.value) return store.articles;
    const lower = searchQuery.value.toLowerCase();
    return store.articles.filter(a => 
        (a.title && a.title.toLowerCase().includes(lower)) || 
        (a.feed_title && a.feed_title.toLowerCase().includes(lower))
    );
});

function onArticleContextMenu(e, article) {
    e.preventDefault();
    e.stopPropagation();
    window.dispatchEvent(new CustomEvent('open-context-menu', {
        detail: {
            x: e.clientX,
            y: e.clientY,
            items: [
                { label: article.is_read ? store.i18n.t('markAsUnread') : store.i18n.t('markAsRead'), action: 'toggleRead', icon: article.is_read ? 'ph-envelope' : 'ph-envelope-open' },
                { label: article.is_favorite ? store.i18n.t('removeFromFavorites') : store.i18n.t('addToFavorites'), action: 'toggleFavorite', icon: article.is_favorite ? 'ph-star-fill' : 'ph-star' },
                { separator: true },
                { label: store.i18n.t('openInBrowser'), action: 'openBrowser', icon: 'ph-arrow-square-out' }
            ],
            data: article,
            callback: handleArticleAction
        }
    }));
}

function handleArticleAction(action, article) {
    if (action === 'toggleRead') {
        const newState = !article.is_read;
        article.is_read = newState;
        fetch(`/api/articles/read?id=${article.id}&read=${newState}`, { method: 'POST' });
    } else if (action === 'toggleFavorite') {
        const newState = !article.is_favorite;
        article.is_favorite = newState;
        fetch(`/api/articles/favorite?id=${article.id}`, { method: 'POST' });
    } else if (action === 'openBrowser') {
        BrowserOpenURL(article.url);
    }
}

async function refreshArticles() {
    await store.refreshFeeds();
    if (listRef.value) {
        listRef.value.scrollTop = 0;
    }
}

</script>

<template>
    <section class="article-list flex flex-col w-full border-r border-border bg-bg-primary shrink-0 h-full">
        <div class="p-4 border-b border-border bg-bg-primary">
            <div class="flex items-center justify-between mb-3">
                <h3 class="m-0 text-lg font-semibold">{{ store.i18n.t('articles') }}</h3>
                <div class="flex items-center gap-2">
                    <div class="relative">
                        <button @click="refreshArticles" class="text-text-secondary hover:text-text-primary hover:bg-bg-tertiary p-1.5 rounded transition-colors" :title="store.i18n.t('refresh')">
                            <i :class="['ph ph-arrow-clockwise text-xl transition-transform', store.refreshProgress.isRunning ? 'animate-spin' : '']"></i>
                        </button>
                        <div v-if="store.refreshProgress.isRunning && store.refreshProgress.total > store.refreshProgress.current" class="absolute -top-1 -right-1 bg-accent text-white text-[10px] font-bold rounded-full min-w-[16px] h-4 px-1 flex items-center justify-center">
                            {{ store.refreshProgress.total - store.refreshProgress.current }}
                        </div>
                    </div>
                    <button @click="emit('toggleSidebar')" class="md:hidden text-2xl p-1">
                        <i class="ph ph-list"></i>
                    </button>
                </div>
            </div>
            <div class="flex items-center bg-bg-secondary border border-border rounded-lg px-3 py-2 focus-within:border-accent transition-colors">
                <i class="ph ph-magnifying-glass text-text-secondary"></i>
                <input type="text" v-model="searchQuery" :placeholder="store.i18n.t('search')" class="bg-transparent border-none outline-none w-full ml-2 text-text-primary text-sm">
            </div>
        </div>
        
        <div class="flex-1 overflow-y-auto" @scroll="handleScroll" ref="listRef">
            <div v-if="filteredArticles.length === 0 && !store.isLoading" class="p-5 text-center text-text-secondary">
                {{ store.i18n.t('noArticles') }}
            </div>
            
            <div v-for="article in filteredArticles" :key="article.id" 
                 :data-article-id="article.id"
                 :ref="el => observeArticle(el)"
                 @click="selectArticle(article)"
                 @contextmenu="onArticleContextMenu($event, article)"
                 :class="['article-card', article.is_read ? 'read' : '', article.is_favorite ? 'favorite' : '', store.currentArticleId === article.id ? 'active' : '']">
                
                <img v-if="article.image_url" :src="article.image_url" class="w-20 h-[60px] object-cover rounded bg-bg-tertiary shrink-0 border border-border" @error="$event.target.style.display='none'">
                
                <div class="flex-1 min-w-0">
                    <h4 v-if="!article.translated_title || article.translated_title === article.title" class="m-0 mb-1.5 text-base font-semibold leading-snug text-text-primary">{{ article.title }}</h4>
                    <div v-else>
                        <h4 class="m-0 mb-1 text-base font-semibold leading-snug text-text-primary">{{ article.translated_title }}</h4>
                        <div class="text-xs text-text-secondary italic mb-1">{{ article.title }}</div>
                    </div>

                    <div class="flex justify-between items-center text-xs text-text-secondary mt-2">
                        <span class="font-medium text-accent">{{ article.feed_title }}</span>
                        <span>{{ formatDate(article.published_at) }}</span>
                    </div>
                    <i v-if="article.is_favorite" class="ph ph-star-fill text-yellow-400 mt-1 block"></i>
                </div>
            </div>
            
            <div v-if="store.isLoading" class="p-4 text-center text-text-secondary">
                <i class="ph ph-spinner ph-spin text-xl"></i>
            </div>
        </div>
    </section>
</template>

<style scoped>
@media (min-width: 768px) {
    .article-list {
        width: var(--article-list-width, 400px);
    }
}
.article-card {
    @apply p-3 border-b border-border cursor-pointer transition-colors flex gap-3 relative hover:bg-bg-tertiary;
}
.article-card.active {
    @apply bg-bg-tertiary border-l-[3px] border-l-accent;
}
.article-card.read h4 {
    @apply text-text-secondary font-normal;
}
.article-card.read .text-sm {
    @apply text-text-secondary opacity-80;
}
.article-card.favorite {
    background-color: rgba(255, 215, 0, 0.05);
}
</style>
