import { computed, ref } from 'vue'

function readNumber(key: string, fallback: number) {
  const raw = localStorage.getItem(key)
  if (!raw) return fallback
  const n = Number(raw)
  return Number.isFinite(n) ? n : fallback
}

function readBool(key: string) {
  return localStorage.getItem(key) === '1'
}

function clamp(n: number, min: number, max: number) {
  return Math.min(max, Math.max(min, n))
}

type ResizeAxis = 'col' | 'row'

export function usePaneResize(options: {
  storageKey: string
  defaultSize: number
  min: number
  max: number | (() => number)
  collapseKey?: string
  collapsedSize?: number
}) {
  const size = ref(readNumber(options.storageKey, options.defaultSize))
  const collapsed = ref(options.collapseKey ? readBool(options.collapseKey) : false)
  const collapsedSize = options.collapsedSize ?? options.min

  const effectiveSize = computed(() => (collapsed.value ? collapsedSize : size.value))

  function persistSize() {
    localStorage.setItem(options.storageKey, String(size.value))
  }

  function bindResize(
    onMove: (ev: MouseEvent) => void,
    onEnd: () => void,
    cursor: 'col-resize' | 'row-resize',
  ) {
    const handleMove = (ev: MouseEvent) => onMove(ev)
    const handleUp = () => {
      document.removeEventListener('mousemove', handleMove)
      document.removeEventListener('mouseup', handleUp)
      document.body.style.cursor = ''
      document.body.style.userSelect = ''
      onEnd()
    }
    document.body.style.cursor = cursor
    document.body.style.userSelect = 'none'
    document.addEventListener('mousemove', handleMove)
    document.addEventListener('mouseup', handleUp)
  }

  function startResize(e: MouseEvent, axis: ResizeAxis) {
    if (collapsed.value) return
    e.preventDefault()
    const max = typeof options.max === 'function' ? options.max() : options.max
    if (axis === 'col') {
      const startX = e.clientX
      const startW = size.value
      bindResize(
        (ev) => {
          size.value = clamp(startW + ev.clientX - startX, options.min, max)
        },
        persistSize,
        'col-resize',
      )
      return
    }
    const startY = e.clientY
    const startH = size.value
    bindResize(
      (ev) => {
        size.value = clamp(startH + (startY - ev.clientY), options.min, max)
      },
      persistSize,
      'row-resize',
    )
  }

  function reset() {
    size.value = options.defaultSize
    persistSize()
  }

  function toggleCollapse() {
    if (!options.collapseKey) return
    collapsed.value = !collapsed.value
    localStorage.setItem(options.collapseKey, collapsed.value ? '1' : '0')
  }

  return {
    size,
    collapsed,
    effectiveSize,
    startResize,
    reset,
    toggleCollapse,
    persistSize,
  }
}
