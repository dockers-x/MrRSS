<script setup lang="ts">
import { ref, watch, onMounted, computed, nextTick } from 'vue';
import { useI18n } from 'vue-i18n';
import type { Article } from '@/types/models';
import ArticleTitle from './parts/ArticleTitle.vue';
import ArticleSummary from './parts/ArticleSummary.vue';
import ArticleLoading from './parts/ArticleLoading.vue';
import ArticleBody from './parts/ArticleBody.vue';
import AudioPlayer from './parts/AudioPlayer.vue';
import VideoPlayer from './parts/VideoPlayer.vue';
import { useArticleSummary } from '@/composables/article/useArticleSummary';
import { useArticleTranslation } from '@/composables/article/useArticleTranslation';
import { useArticleRendering } from '@/composables/article/useArticleRendering';
import './ArticleContent.css';

interface SummaryResult {
  summary: string;
  sentence_count: number;
  is_too_short: boolean;
  error?: string;
}

interface Props {
  article: Article;
  articleContent: string;
  isLoadingContent: boolean;
}

const props = defineProps<Props>();

const { t } = useI18n();

// Use composables for summary and translation
const {
  summarySettings,
  loadSummarySettings,
  generateSummary: generateSummaryComposable,
  isSummaryLoading,
} = useArticleSummary();

const { translationSettings, loadTranslationSettings } = useArticleTranslation();

// Use composable for enhanced rendering (math formulas, etc.)
const { enhanceRendering } = useArticleRendering();

// Computed properties for easier access
const summaryEnabled = computed(() => summarySettings.value.enabled);
const translationEnabled = computed(() => translationSettings.value.enabled);
const targetLanguage = computed(() => translationSettings.value.targetLang);

// Current article summary
const summaryResult = ref<SummaryResult | null>(null);
const isLoadingSummary = computed(() =>
  props.article ? isSummaryLoading(props.article.id) : false
);

// Additional state for summary translation
const translatedSummary = ref('');
const isTranslatingSummary = ref(false);

// Additional state for translation
const translatedTitle = ref('');
const isTranslatingTitle = ref(false);
const isTranslatingContent = ref(false);
const lastTranslatedArticleId = ref<number | null>(null);

// Load settings using composables
async function loadSettings() {
  await loadSummarySettings();
  await loadTranslationSettings();
}

// Translate text using the API
async function translateText(text: string): Promise<string> {
  if (!text || !translationEnabled.value) return '';

  try {
    const res = await fetch('/api/articles/translate-text', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        text: text,
        target_language: targetLanguage.value,
      }),
    });

    if (res.ok) {
      const data = await res.json();
      return data.translated_text || '';
    } else {
      console.error('Error translating text:', res.status);
      window.showToast(t('errorTranslatingContent'), 'error');
    }
  } catch (e) {
    console.error('Error translating text:', e);
    window.showToast(t('errorTranslating'), 'error');
  }
  return '';
}

// Generate summary for the current article
async function generateSummary(article: Article) {
  if (!summaryEnabled.value || !article) {
    return;
  }

  summaryResult.value = null;
  translatedSummary.value = '';

  const result = await generateSummaryComposable(article);
  summaryResult.value = result;

  // Auto-translate summary if translation is enabled
  if (translationEnabled.value && result?.summary && !result.is_too_short) {
    isTranslatingSummary.value = true;
    translatedSummary.value = await translateText(result.summary);
    isTranslatingSummary.value = false;
  }
}

// Translate title
async function translateTitle(article: Article) {
  if (!translationEnabled.value || !article?.title) return;

  isTranslatingTitle.value = true;
  translatedTitle.value = await translateText(article.title);
  isTranslatingTitle.value = false;
}

// Translate content paragraphs
async function translateContentParagraphs(content: string) {
  if (!translationEnabled.value || !content) return;

  // Prevent duplicate translations for the same article
  if (lastTranslatedArticleId.value === props.article?.id) {
    return;
  }

  isTranslatingContent.value = true;
  lastTranslatedArticleId.value = props.article?.id || null;

  // Wait for content to render
  await nextTick();

  // Find all text elements in the prose content
  const proseContainer = document.querySelector('.prose-content');
  if (!proseContainer) {
    isTranslatingContent.value = false;
    return;
  }

  // Remove any existing translations first
  const existingTranslations = proseContainer.querySelectorAll('.translation-text');
  existingTranslations.forEach((el) => el.remove());

  // Find all translatable elements (only top-level text elements)
  const textTags = ['P', 'H1', 'H2', 'H3', 'H4', 'H5', 'H6', 'LI', 'BLOCKQUOTE', 'FIGCAPTION'];
  const elements = proseContainer.querySelectorAll(textTags.join(','));

  for (const el of elements) {
    const htmlEl = el as HTMLElement;

    // Skip if inside a translation element
    if (htmlEl.closest('.translation-text')) continue;

    // Skip if already has translation sibling
    if (htmlEl.nextElementSibling?.classList.contains('translation-text')) continue;

    // Skip technical content that should not be translated:
    // - Code blocks (pre, code)
    // - Keyboard input (kbd)
    // - Math formulas (katex)
    // - Elements with specific classes
    if (
      htmlEl.closest('pre') ||
      htmlEl.closest('code') ||
      htmlEl.closest('kbd') ||
      htmlEl.closest('.katex') ||
      htmlEl.closest('.katex-display') ||
      htmlEl.closest('.katex-inline') ||
      htmlEl.classList.contains('katex') ||
      htmlEl.classList.contains('katex-display') ||
      htmlEl.classList.contains('katex-inline')
    ) {
      continue;
    }

    // Get the visible text content for translation
    // For elements containing code/kbd/math inline, extract only translatable text
    let visibleText = '';

    // Check if element contains technical elements
    const hasTechnicalContent = htmlEl.querySelector('code, kbd, .katex, .katex-inline');

    if (hasTechnicalContent) {
      // Extract only text nodes, skipping technical elements
      const textNodes: string[] = [];
      const walker = document.createTreeWalker(htmlEl, NodeFilter.SHOW_TEXT, {
        acceptNode: (node) => {
          const parent = node.parentElement;
          if (
            parent?.tagName === 'CODE' ||
            parent?.tagName === 'KBD' ||
            parent?.classList.contains('katex') ||
            parent?.classList.contains('katex-inline')
          ) {
            return NodeFilter.FILTER_REJECT;
          }
          return NodeFilter.FILTER_ACCEPT;
        },
      });

      let node;
      while ((node = walker.nextNode())) {
        const text = node.textContent?.trim();
        if (text) textNodes.push(text);
      }

      visibleText = textNodes.join(' ').trim();
    } else {
      visibleText = htmlEl.textContent?.trim() || '';
    }

    if (!visibleText || visibleText.length < 2) continue;

    // Translate the visible text
    const translated = await translateText(visibleText);

    // Skip if translation is same as original or empty
    if (!translated || translated === visibleText) continue;

    // Create translation element with same tag type
    const translationEl = document.createElement(htmlEl.tagName.toLowerCase());
    translationEl.className = 'translation-text';
    translationEl.textContent = translated;

    // Copy blockquote styling
    if (htmlEl.tagName === 'BLOCKQUOTE') {
      translationEl.style.borderLeft = '4px solid var(--accent-color)';
      translationEl.style.paddingLeft = '1em';
      translationEl.style.fontStyle = 'italic';
    }

    // Insert translation after original
    htmlEl.parentNode?.insertBefore(translationEl, htmlEl.nextSibling);
  }

  isTranslatingContent.value = false;
}

// Watch for article changes and regenerate summary + translations
watch(
  () => props.article?.id,
  (newId, oldId) => {
    if (newId !== oldId) {
      summaryResult.value = null;
      translatedSummary.value = '';
      translatedTitle.value = '';
      lastTranslatedArticleId.value = null; // Reset translation tracking

      if (props.article) {
        if (summaryEnabled.value) {
          generateSummary(props.article);
        }
        if (translationEnabled.value) {
          translateTitle(props.article);
        }
      }
    }
  }
);

// Watch for content loading completion only
watch(
  () => props.isLoadingContent,
  (isLoading, wasLoading) => {
    if (wasLoading && !isLoading && props.article) {
      // Enhance rendering first (math formulas, etc.)
      nextTick(() => {
        enhanceRendering('.prose-content');
      });

      if (summaryEnabled.value) {
        generateSummary(props.article);
      }
      if (translationEnabled.value && lastTranslatedArticleId.value !== props.article.id) {
        nextTick(() => translateContentParagraphs(props.articleContent));
      }
    }
  }
);

onMounted(async () => {
  await loadSettings();
  if (props.article) {
    // Enhance rendering if content is already loaded
    if (props.articleContent && !props.isLoadingContent) {
      await nextTick();
      enhanceRendering('.prose-content');
    }

    if (summaryEnabled.value && props.articleContent) {
      generateSummary(props.article);
    }
    if (translationEnabled.value) {
      translateTitle(props.article);
      if (props.articleContent && !props.isLoadingContent) {
        nextTick(() => translateContentParagraphs(props.articleContent));
      }
    }
  }
});
</script>

<template>
  <div class="flex-1 overflow-y-auto bg-bg-primary p-3 sm:p-6">
    <div class="max-w-3xl mx-auto bg-bg-primary">
      <ArticleTitle
        :article="article"
        :translated-title="translatedTitle"
        :is-translating-title="isTranslatingTitle"
        :translation-enabled="translationEnabled"
      />

      <!-- Audio Player (if article has audio) -->
      <AudioPlayer
        v-if="article.audio_url"
        :audio-url="article.audio_url"
        :article-title="article.title"
      />

      <!-- Video Player (if article has video) -->
      <VideoPlayer
        v-if="article.video_url"
        :video-url="article.video_url"
        :article-title="article.title"
      />

      <ArticleSummary
        :summary-result="summaryResult"
        :is-loading-summary="isLoadingSummary"
        :translated-summary="translatedSummary"
        :is-translating-summary="isTranslatingSummary"
        :translation-enabled="translationEnabled"
      />

      <ArticleLoading v-if="isLoadingContent" />

      <ArticleBody
        v-else
        :article-content="articleContent"
        :is-translating-content="isTranslatingContent"
        :has-media-content="!!(article.audio_url || article.video_url)"
      />
    </div>
  </div>
</template>
