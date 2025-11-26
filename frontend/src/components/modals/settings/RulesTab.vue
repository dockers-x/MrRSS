<script setup>
import { store } from '../../../store.js';
import { ref, onMounted, watch } from 'vue';
import { 
    PhLightning, PhPlus, PhTrash, PhPencil,
    PhPlay, PhFunnel, PhListChecks
} from "@phosphor-icons/vue";
import RuleEditorModal from '../RuleEditorModal.vue';

const props = defineProps({
    settings: { type: Object, required: true }
});

// Rules list
const rules = ref([]);

// Modal states
const showRuleEditor = ref(false);
const editingRule = ref(null);
const applyingRuleId = ref(null);

// Load rules from settings
onMounted(() => {
    loadRules();
});

function loadRules() {
    if (props.settings.rules) {
        try {
            const parsed = typeof props.settings.rules === 'string' 
                ? JSON.parse(props.settings.rules) 
                : props.settings.rules;
            rules.value = Array.isArray(parsed) ? parsed : [];
        } catch (e) {
            console.error('Error parsing rules:', e);
            rules.value = [];
        }
    }
}

// Watch for settings changes
watch(() => props.settings.rules, () => {
    loadRules();
}, { immediate: true });

// Save rules to settings
async function saveRules() {
    try {
        props.settings.rules = JSON.stringify(rules.value);
        await fetch('/api/settings', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ rules: props.settings.rules })
        });
    } catch (e) {
        console.error('Error saving rules:', e);
    }
}

// Add new rule
function addRule() {
    editingRule.value = null;
    showRuleEditor.value = true;
}

// Edit existing rule
function editRule(rule) {
    editingRule.value = { ...rule };
    showRuleEditor.value = true;
}

// Delete rule
async function deleteRule(ruleId) {
    const confirmed = await window.showConfirm({
        title: store.i18n.t('ruleDeleteConfirmTitle'),
        message: store.i18n.t('ruleDeleteConfirmMessage'),
        confirmText: store.i18n.t('delete'),
        cancelText: store.i18n.t('cancel'),
        isDanger: true
    });
    
    if (!confirmed) return;
    
    rules.value = rules.value.filter(r => r.id !== ruleId);
    await saveRules();
    window.showToast(store.i18n.t('ruleDeletedSuccess'), 'success');
}

// Toggle rule enabled state
async function toggleRuleEnabled(rule) {
    rule.enabled = !rule.enabled;
    await saveRules();
}

// Save rule from editor
async function handleSaveRule(rule) {
    // Check if this is a new rule (editingRule is null or has no id)
    const isNew = !editingRule.value || !editingRule.value.id;
    
    if (editingRule.value && editingRule.value.id) {
        // Update existing rule
        const index = rules.value.findIndex(r => r.id === rule.id);
        if (index !== -1) {
            rules.value[index] = rule;
        }
    } else {
        // Add new rule
        rule.id = Date.now();
        rule.enabled = true;
        rules.value.push(rule);
    }
    
    await saveRules();
    showRuleEditor.value = false;
    window.showToast(store.i18n.t('ruleSavedSuccess'), 'success');
    
    // Apply rule to existing articles when adding a new rule
    if (isNew && rule.enabled) {
        await applyRule(rule);
    }
}

// Apply rule now
async function applyRule(rule) {
    applyingRuleId.value = rule.id;
    
    try {
        const res = await fetch('/api/rules/apply', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(rule)
        });
        
        if (res.ok) {
            const data = await res.json();
            window.showToast(store.i18n.t('ruleAppliedSuccess', { count: data.affected }), 'success');
            store.fetchArticles();
            store.fetchUnreadCounts();
        } else {
            window.showToast(store.i18n.t('errorSavingSettings'), 'error');
        }
    } catch (e) {
        console.error('Error applying rule:', e);
        window.showToast(store.i18n.t('errorSavingSettings'), 'error');
    } finally {
        applyingRuleId.value = null;
    }
}

// Format condition for display
function formatCondition(rule) {
    if (!rule.conditions || rule.conditions.length === 0) {
        return store.i18n.t('conditionAlways');
    }
    
    // Simplified display - show first condition
    const first = rule.conditions[0];
    let text = formatSingleCondition(first);
    
    if (rule.conditions.length > 1) {
        text += ` ${store.i18n.t('andNMore', { count: rule.conditions.length - 1 })}`;
    }
    
    return text;
}

function formatSingleCondition(condition) {
    const fieldLabels = {
        'feed_name': store.i18n.t('feedName'),
        'feed_category': store.i18n.t('feedCategory'),
        'article_title': store.i18n.t('articleTitle'),
        'published_after': store.i18n.t('publishedAfter'),
        'published_before': store.i18n.t('publishedBefore'),
        'is_read': store.i18n.t('readStatus'),
        'is_favorite': store.i18n.t('favoriteStatus'),
        'is_hidden': store.i18n.t('hiddenStatus')
    };
    
    const field = fieldLabels[condition.field] || condition.field;
    let value = condition.value || (condition.values && condition.values.length > 0 ? condition.values[0] : '');
    
    if (condition.negate) {
        return `${store.i18n.t('not')} ${field}: ${value}`;
    }
    
    return `${field}: ${value}`;
}

// Format actions for display
function formatActions(rule) {
    if (!rule.actions || rule.actions.length === 0) {
        return '-';
    }
    
    const actionLabels = {
        'favorite': store.i18n.t('actionFavorite'),
        'unfavorite': store.i18n.t('actionUnfavorite'),
        'hide': store.i18n.t('actionHide'),
        'unhide': store.i18n.t('actionUnhide'),
        'mark_read': store.i18n.t('actionMarkRead'),
        'mark_unread': store.i18n.t('actionMarkUnread')
    };
    
    return rule.actions.map(a => actionLabels[a] || a).join(', ');
}
</script>

<template>
    <div class="space-y-4 sm:space-y-6">
        <div class="setting-group">
            <label class="font-semibold mb-2 sm:mb-3 text-text-secondary uppercase text-xs tracking-wider flex items-center gap-2">
                <PhLightning :size="14" class="sm:w-4 sm:h-4" />
                {{ store.i18n.t('rules') }}
            </label>
            
            <!-- Header with description and add button -->
            <div class="setting-item mb-2 sm:mb-3">
                <div class="flex-1 flex items-start gap-2 sm:gap-3 min-w-0">
                    <PhLightning :size="20" class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6" />
                    <div class="flex-1 min-w-0">
                        <div class="font-medium mb-1 text-sm sm:text-base">{{ store.i18n.t('rules') }}</div>
                        <div class="text-xs text-text-secondary hidden sm:block">{{ store.i18n.t('rulesDesc') }}</div>
                    </div>
                </div>
                <button @click="addRule" class="btn-primary">
                    <PhPlus :size="16" class="sm:w-5 sm:h-5" />
                    <span class="hidden sm:inline">{{ store.i18n.t('addRule') }}</span>
                </button>
            </div>
            
            <!-- Empty state -->
            <div v-if="rules.length === 0" class="empty-state">
                <PhLightning :size="48" class="mx-auto mb-3 opacity-30" />
                <p class="text-text-secondary text-sm sm:text-base">{{ store.i18n.t('noRules') }}</p>
                <p class="text-text-secondary text-xs mt-1">{{ store.i18n.t('noRulesHint') }}</p>
            </div>
            
            <!-- Rules List -->
            <div v-else class="space-y-2 sm:space-y-3">
                <div v-for="rule in rules" :key="rule.id" class="rule-item">
                    <div class="flex items-start gap-2 sm:gap-4">
                        <!-- Toggle and Info -->
                        <div class="flex-1 flex items-start gap-2 sm:gap-3 min-w-0">
                            <input 
                                type="checkbox" 
                                :checked="rule.enabled" 
                                @change="toggleRuleEnabled(rule)"
                                class="toggle mt-1"
                            >
                            <div class="flex-1 min-w-0">
                                <div class="font-medium mb-1 text-sm sm:text-base truncate" :class="{ 'text-text-secondary': !rule.enabled }">
                                    {{ rule.name || store.i18n.t('rules') + ' #' + rule.id }}
                                </div>
                                <div class="text-xs text-text-secondary flex flex-wrap items-center gap-1 sm:gap-2">
                                    <span class="condition-badge">
                                        <PhFunnel :size="12" />
                                        {{ formatCondition(rule) }}
                                    </span>
                                    <span class="text-text-tertiary">→</span>
                                    <span class="action-badge">
                                        <PhListChecks :size="12" />
                                        {{ formatActions(rule) }}
                                    </span>
                                </div>
                            </div>
                        </div>
                        
                        <!-- Action buttons -->
                        <div class="flex items-center gap-1 sm:gap-2 shrink-0">
                            <button 
                                @click="applyRule(rule)" 
                                class="action-btn" 
                                :disabled="applyingRuleId === rule.id"
                                :title="store.i18n.t('applyRuleNow')"
                            >
                                <PhPlay v-if="applyingRuleId !== rule.id" :size="18" class="sm:w-5 sm:h-5" />
                                <span v-else class="animate-spin text-sm">⟳</span>
                            </button>
                            <button @click="editRule(rule)" class="action-btn" :title="store.i18n.t('editRule')">
                                <PhPencil :size="18" class="sm:w-5 sm:h-5" />
                            </button>
                            <button @click="deleteRule(rule.id)" class="action-btn danger" :title="store.i18n.t('deleteRule')">
                                <PhTrash :size="18" class="sm:w-5 sm:h-5" />
                            </button>
                        </div>
                    </div>
                </div>
            </div>
        </div>
        
        <!-- Rule Editor Modal -->
        <RuleEditorModal 
            v-if="showRuleEditor"
            :show="showRuleEditor"
            :rule="editingRule"
            @close="showRuleEditor = false"
            @save="handleSaveRule"
        />
    </div>
</template>

<style scoped>
.setting-item {
    @apply flex items-start justify-between gap-2 sm:gap-4 p-2 sm:p-3 rounded-lg bg-bg-secondary border border-border;
}

.rule-item {
    @apply p-2 sm:p-3 rounded-lg bg-bg-secondary border border-border;
}

.toggle {
    @apply w-10 h-5 appearance-none bg-bg-tertiary rounded-full relative cursor-pointer border border-border transition-colors checked:bg-accent checked:border-accent shrink-0;
}
.toggle::after {
    content: '';
    @apply absolute top-0.5 left-0.5 w-3.5 h-3.5 bg-white rounded-full shadow-sm transition-transform;
}
.toggle:checked::after {
    transform: translateX(20px);
}

.btn-primary {
    @apply bg-accent text-white border-none px-3 py-2 sm:px-4 sm:py-2.5 rounded-lg cursor-pointer flex items-center gap-1 sm:gap-2 font-medium hover:bg-accent-hover transition-colors text-sm sm:text-base;
}

.empty-state {
    @apply text-center py-8 sm:py-12;
}

.condition-badge, .action-badge {
    @apply inline-flex items-center gap-1 px-1.5 sm:px-2 py-0.5 sm:py-1 rounded text-[10px] sm:text-xs bg-bg-tertiary;
}

.action-btn {
    @apply p-1.5 sm:p-2 rounded-lg bg-transparent border-none cursor-pointer text-text-secondary hover:bg-bg-tertiary hover:text-text-primary transition-colors;
}

.action-btn.danger:hover {
    @apply text-red-500 bg-red-500/10;
}

.action-btn:disabled {
    @apply opacity-50 cursor-not-allowed;
}

.animate-spin {
    animation: spin 1s linear infinite;
    display: inline-block;
}

@keyframes spin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
}
</style>
