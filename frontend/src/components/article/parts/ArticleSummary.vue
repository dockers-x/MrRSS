<script setup lang="ts">
import { ref } from 'vue';
import { PhTextAlignLeft, PhSpinnerGap } from '@phosphor-icons/vue';
import { useI18n } from 'vue-i18n';

interface Props {
  summaryResult: {
    summary: string;
    sentence_count: number;
    is_too_short: boolean;
    error?: string;
  } | null;
  isLoadingSummary: boolean;
  translatedSummary: string;
  isTranslatingSummary: boolean;
  translationEnabled: boolean;
}

defineProps<Props>();

const { t } = useI18n();

const showSummary = ref(true);
</script>

<template>
  <!-- Summary Section -->
  <div
    v-if="summaryResult || isLoadingSummary"
    class="mb-6 p-4 rounded-lg border border-border bg-bg-secondary"
  >
    <!-- Summary Header -->
    <div
      class="flex items-center justify-between cursor-pointer"
      @click="showSummary = !showSummary"
    >
      <div class="flex items-center gap-2 text-accent font-medium">
        <PhTextAlignLeft :size="20" />
        <span>{{ t('articleSummary') }}</span>
      </div>
      <span class="text-xs text-text-secondary">
        {{ showSummary ? '▲' : '▼' }}
      </span>
    </div>

    <!-- Summary Content -->
    <div v-if="showSummary" class="mt-3">
      <!-- Loading State -->
      <div v-if="isLoadingSummary" class="flex items-center gap-2 text-text-secondary">
        <PhSpinnerGap :size="16" class="animate-spin" />
        <span class="text-sm">{{ t('generatingSummary') }}</span>
      </div>

      <!-- Too Short Warning -->
      <div v-else-if="summaryResult?.is_too_short" class="text-sm text-text-secondary italic">
        {{ t('summaryTooShort') }}
      </div>

      <!-- Summary Display -->
      <div v-else-if="summaryResult?.summary">
        <!-- Show translated summary only when translation is enabled -->
        <div
          v-if="translationEnabled && translatedSummary"
          class="text-sm text-text-primary leading-relaxed select-text"
        >
          {{ translatedSummary }}
        </div>
        <!-- Show original summary when no translation or as fallback -->
        <p v-else class="text-sm text-text-primary leading-relaxed select-text">
          {{ summaryResult.summary }}
        </p>
        <!-- Translation loading indicator -->
        <div v-if="isTranslatingSummary" class="flex items-center gap-1 mt-2 text-text-secondary">
          <PhSpinnerGap :size="12" class="animate-spin" />
          <span class="text-xs">{{ t('translating') }}</span>
        </div>
      </div>

      <!-- No Summary Available -->
      <div v-else class="text-sm text-text-secondary italic">{{ t('noSummaryAvailable') }}</div>
    </div>
  </div>
</template>
