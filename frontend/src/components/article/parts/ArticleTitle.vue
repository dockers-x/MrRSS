<script setup lang="ts">
import { computed } from 'vue';
import { PhSpinnerGap, PhTranslate } from '@phosphor-icons/vue';
import type { Article } from '@/types/models';
import { formatDate } from '@/utils/date';
import { useI18n } from 'vue-i18n';

interface Props {
  article: Article;
  translatedTitle: string;
  isTranslatingTitle: boolean;
  translationEnabled: boolean;
}

const props = defineProps<Props>();

const { t } = useI18n();
const { locale } = useI18n();

// Translation function wrapper for formatDate
const formatDateWithI18n = (dateStr: string): string => {
  return formatDate(dateStr, locale.value, t);
};

// Computed: check if we should show bilingual title
const showBilingualTitle = computed(() => {
  return (
    props.translationEnabled &&
    props.translatedTitle &&
    props.translatedTitle !== props.article?.title
  );
});
</script>

<template>
  <!-- Title Section - Bilingual when translation enabled -->
  <div class="mb-3 sm:mb-4">
    <!-- Original Title -->
    <h1 class="text-xl sm:text-3xl font-bold leading-tight text-text-primary select-text">
      {{ article.title }}
    </h1>
    <!-- Translated Title (shown below if different from original) -->
    <h2
      v-if="showBilingualTitle"
      class="text-base sm:text-xl font-medium leading-tight mt-2 text-text-secondary select-text"
    >
      {{ translatedTitle }}
    </h2>
    <!-- Translation loading indicator for title -->
    <div v-if="isTranslatingTitle" class="flex items-center gap-1 mt-1 text-text-secondary">
      <PhSpinnerGap :size="12" class="animate-spin" />
      <span class="text-xs">Translating...</span>
    </div>
  </div>

  <div
    class="text-xs sm:text-sm text-text-secondary mb-4 sm:mb-6 flex flex-wrap items-center gap-2 sm:gap-4"
  >
    <span>{{ article.feed_title }}</span>
    <span class="hidden sm:inline">•</span>
    <span>{{ formatDateWithI18n(article.published_at) }}</span>
    <span v-if="translationEnabled" class="flex items-center gap-1 text-accent">
      <PhTranslate :size="14" />
      <span class="text-xs">{{ t('autoTranslateEnabled') }}</span>
    </span>
  </div>
</template>
