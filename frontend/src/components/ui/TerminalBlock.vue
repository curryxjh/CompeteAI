<script setup lang="ts">
import { ref } from 'vue'

const props = withDefaults(
  defineProps<{
    title: string
    content: string
    lang?: string
  }>(),
  { lang: 'json' },
)

const copied = ref(false)

async function copy() {
  await navigator.clipboard.writeText(props.content)
  copied.value = true
  setTimeout(() => {
    copied.value = false
  }, 1500)
}
</script>

<template>
  <div class="terminal-block">
    <div class="terminal-header">
      <span class="terminal-title font-mono">{{ title }}</span>
      <span class="terminal-lang">{{ lang }}</span>
      <button type="button" class="copy-btn font-mono" @click="copy">
        {{ copied ? 'copied' : 'copy' }}
      </button>
    </div>
    <pre class="terminal-body font-mono"><code>{{ content }}</code></pre>
  </div>
</template>

<style scoped>
.terminal-block {
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  background: var(--bg-terminal);
  overflow: hidden;
}
.terminal-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border-bottom: 1px solid var(--border-subtle);
  background: rgba(255, 255, 255, 0.04);
}
.terminal-title {
  font-size: 11px;
  color: var(--text-secondary);
  letter-spacing: 0.02em;
}
.terminal-lang {
  margin-left: auto;
  font-size: 10px;
  color: var(--text-muted);
  text-transform: uppercase;
}
.copy-btn {
  font-size: 10px;
  color: var(--text-muted);
  background: none;
  border: none;
  cursor: pointer;
  padding: 2px 6px;
  border-radius: 4px;
}
.copy-btn:hover {
  color: var(--accent);
  background: var(--accent-dim);
}
.terminal-body {
  margin: 0;
  padding: 12px 14px;
  font-size: 12px;
  line-height: 1.6;
  color: #c4c4cc;
  white-space: pre-wrap;
  word-break: break-word;
  max-height: 320px;
  overflow: auto;
}
</style>
