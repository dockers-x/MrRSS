import { nextTick } from 'vue';
import katex from 'katex';

/**
 * Composable for enhanced article content rendering
 * Handles math formulas, code syntax highlighting, and other advanced rendering
 */
export function useArticleRendering() {
  /**
   * Render math formulas in the content
   * Supports both inline math $...$ and display math $$...$$
   */
  function renderMathFormulas(container: HTMLElement) {
    if (!container) return;

    try {
      // Find all text nodes that might contain math
      const walker = document.createTreeWalker(container, NodeFilter.SHOW_TEXT, {
        acceptNode: (node) => {
          // Skip if already inside a math element
          const parent = node.parentElement;
          if (
            parent?.classList.contains('katex') ||
            parent?.classList.contains('katex-display') ||
            parent?.tagName === 'CODE' ||
            parent?.tagName === 'PRE' ||
            parent?.tagName === 'SCRIPT'
          ) {
            return NodeFilter.FILTER_REJECT;
          }
          // Only accept nodes that contain $ symbols
          if (node.textContent?.includes('$')) {
            return NodeFilter.FILTER_ACCEPT;
          }
          return NodeFilter.FILTER_REJECT;
        },
      });

      const nodesToProcess: Text[] = [];
      let currentNode: Node | null;
      while ((currentNode = walker.nextNode())) {
        nodesToProcess.push(currentNode as Text);
      }

      // Process each text node
      for (const node of nodesToProcess) {
        const text = node.textContent || '';
        if (!text.includes('$')) continue;

        const fragments: (string | HTMLElement)[] = [];
        let lastIndex = 0;

        // Regex to match $$...$$ (display math) and $...$ (inline math)
        // Display math takes precedence
        const displayMathRegex = /\$\$([^$]+)\$\$/g;
        const inlineMathRegex = /\$([^$\n]+)\$/g;

        // First, find all display math
        const displayMatches: Array<{
          start: number;
          end: number;
          math: string;
          isDisplay: boolean;
        }> = [];
        let match;
        while ((match = displayMathRegex.exec(text)) !== null) {
          displayMatches.push({
            start: match.index,
            end: match.index + match[0].length,
            math: match[1],
            isDisplay: true,
          });
        }

        // Then find all inline math, but skip those inside display math
        const inlineMatches: Array<{
          start: number;
          end: number;
          math: string;
          isDisplay: boolean;
        }> = [];
        while ((match = inlineMathRegex.exec(text)) !== null) {
          const isInsideDisplay = displayMatches.some(
            (dm) => match!.index >= dm.start && match!.index < dm.end
          );
          if (!isInsideDisplay) {
            inlineMatches.push({
              start: match.index,
              end: match.index + match[0].length,
              math: match[1],
              isDisplay: false,
            });
          }
        }

        // Combine and sort all matches
        const allMatches = [...displayMatches, ...inlineMatches].sort((a, b) => a.start - b.start);

        // Build fragments
        for (const { start, end, math, isDisplay } of allMatches) {
          // Add text before match
          if (start > lastIndex) {
            fragments.push(text.substring(lastIndex, start));
          }

          // Render math
          try {
            const mathElement = document.createElement(isDisplay ? 'div' : 'span');
            mathElement.className = isDisplay ? 'katex-display' : 'katex-inline';
            katex.render(math, mathElement, {
              displayMode: isDisplay,
              throwOnError: false,
              strict: false,
            });
            fragments.push(mathElement);
          } catch (e) {
            console.error('Error rendering math:', e);
            // On error, keep the original text
            fragments.push(text.substring(start, end));
          }

          lastIndex = end;
        }

        // Add remaining text
        if (lastIndex < text.length) {
          fragments.push(text.substring(lastIndex));
        }

        // Only replace if we found any math
        if (fragments.length > 0 && allMatches.length > 0) {
          const parent = node.parentNode;
          if (parent) {
            // Insert fragments
            for (const fragment of fragments) {
              if (typeof fragment === 'string') {
                parent.insertBefore(document.createTextNode(fragment), node);
              } else {
                parent.insertBefore(fragment, node);
              }
            }
            // Remove original text node
            parent.removeChild(node);
          }
        }
      }
    } catch (e) {
      console.error('Error processing math formulas:', e);
    }
  }

  /**
   * Apply all rendering enhancements to the container
   */
  async function enhanceRendering(containerSelector: string = '.prose-content') {
    await nextTick();
    const container = document.querySelector(containerSelector) as HTMLElement;
    if (!container) return;

    // Render math formulas
    renderMathFormulas(container);
  }

  return {
    renderMathFormulas,
    enhanceRendering,
  };
}
