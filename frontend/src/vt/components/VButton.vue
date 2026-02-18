<template>
  <component
    :is="to ? 'router-link' : 'button'"
    :to="to"
    :type="to ? undefined : type"
    :disabled="disabled"
    :class="classes"
  >
    <slot />
  </component>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{
  variant?: 'primary' | 'secondary' | 'danger'
  size?: 'sm' | 'md'
  to?: string
  disabled?: boolean
  type?: string
}>(), {
  variant: 'primary',
  size: 'md',
  type: 'button',
})

const classMap: Record<string, Record<string, string>> = {
  primary: {
    md: 'px-6 py-2.5 bg-accent text-white font-medium rounded-lg hover:bg-accent-hover disabled:opacity-50',
    sm: 'px-4 py-2 bg-accent text-white text-sm font-medium rounded-lg hover:bg-accent-hover transition-colors shrink-0',
  },
  secondary: {
    md: 'px-4 py-2 text-sm font-medium text-fg-secondary border border-edge-strong rounded-lg hover:bg-surface-alt',
    sm: 'px-3 py-1.5 text-xs font-medium text-fg-secondary bg-surface border border-edge-strong rounded-lg hover:bg-surface-alt transition-colors',
  },
  danger: {
    md: 'px-4 py-2 text-sm font-medium text-white bg-danger rounded-lg hover:bg-danger-hover',
  },
}

const classes = computed(() => classMap[props.variant]?.[props.size] ?? '')
</script>
