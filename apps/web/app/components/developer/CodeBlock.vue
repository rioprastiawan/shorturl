<script setup lang="ts">
const props = defineProps<{
  code: string
  language: 'curl' | 'javascript' | 'php' | 'go' | 'json'
}>()

const keywords: Record<string, Set<string>> = {
  javascript: new Set(['const', 'await', 'async', 'fetch', 'JSON', 'stringify', 'console', 'log']),
  php: new Set(['php', 'curl_init', 'curl_setopt_array', 'curl_exec', 'curl_close', 'json_encode', 'echo']),
  go: new Set(['package', 'import', 'func', 'var', 'defer', 'if', 'return', 'nil', 'string', 'http', 'json', 'bytes', 'os', 'fmt', 'io']),
  curl: new Set(['curl']),
  json: new Set(['true', 'false', 'null']),
}

function escape(value: string) {
  return value.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;')
}

function tokenClass(token: string) {
  if (keywords[props.language]?.has(token)) return 'syntax-keyword'
  if (/^--?[A-Za-z]/.test(token)) return 'syntax-property'
  if (/^\$[A-Za-z_]/.test(token)) return 'syntax-variable'
  if (/^\d+$/.test(token)) return 'syntax-number'
  if (/^[A-Z][A-Z0-9_]+$/.test(token)) return 'syntax-constant'
  return ''
}

/** Small, dependency-free lexer for the four snippets displayed here. */
const highlighted = computed(() => props.code.split('\n').map((line) => {
  let output = ''
  let index = 0

  while (index < line.length) {
    const rest = line.slice(index)
    if (rest.startsWith('//') || (props.language !== 'javascript' && rest.startsWith('#'))) {
      output += `<span class="syntax-comment">${escape(rest)}</span>`
      break
    }

    const char = line[index]!
    if (char === '"' || char === "'" || char === '`') {
      let end = index + 1
      while (end < line.length) {
        if (line[end] === char && line[end - 1] !== '\\') {
          end++
          break
        }
        end++
      }
      output += `<span class="syntax-string">${escape(line.slice(index, end))}</span>`
      index = end
      continue
    }

    const match = rest.match(/^(\$?[A-Za-z_][\w-]*|--?[A-Za-z][\w-]*|\d+)/)
    if (match) {
      const token = match[0]
      const className = tokenClass(token)
      output += className ? `<span class="${className}">${escape(token)}</span>` : escape(token)
      index += token.length
      continue
    }

    output += escape(char)
    index++
  }

  return output
}).join('\n'))
</script>

<template>
  <div class="min-w-0 w-full max-w-full overflow-hidden rounded-xl border border-[#30363d] bg-[#0d1117] shadow-inner">
    <div class="flex items-center justify-between border-b border-[#30363d] bg-[#161b22] px-3 py-2">
      <div class="flex gap-1.5" aria-hidden="true">
        <span class="size-2.5 rounded-full bg-[#ff5f56]" />
        <span class="size-2.5 rounded-full bg-[#ffbd2e]" />
        <span class="size-2.5 rounded-full bg-[#27c93f]" />
      </div>
      <UiCopyButton :value="code" label="Copy code" class="text-slate-300 hover:bg-white/10 hover:text-white" />
    </div>
    <pre class="min-w-0 w-full max-w-full overflow-x-auto overscroll-x-contain p-4 text-[12px] leading-6 text-[#c9d1d9]"><code class="font-mono" v-html="highlighted" /></pre>
  </div>
</template>

<style scoped>
:deep(.syntax-keyword) { color: #d2a8ff; }
:deep(.syntax-string) { color: #a5d6ff; }
:deep(.syntax-property) { color: #79c0ff; }
:deep(.syntax-variable) { color: #ffa657; }
:deep(.syntax-number), :deep(.syntax-constant) { color: #79c0ff; }
:deep(.syntax-comment) { color: #8b949e; font-style: italic; }
</style>
