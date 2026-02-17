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
    md: 'px-6 py-2.5 bg-blue-600 text-white font-medium rounded-lg hover:bg-blue-700 disabled:opacity-50',
    sm: 'px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-lg hover:bg-blue-700 transition-colors shrink-0',
  },
  secondary: {
    md: 'px-4 py-2 text-sm font-medium text-gray-600 border border-gray-300 rounded-lg hover:bg-gray-50',
    sm: 'px-3 py-1.5 text-xs font-medium text-gray-700 bg-white border border-gray-300 rounded-lg hover:bg-gray-50 transition-colors',
  },
  danger: {
    md: 'px-4 py-2 text-sm font-medium text-white bg-red-600 rounded-lg hover:bg-red-700',
  },
}

const classes = computed(() => classMap[props.variant]?.[props.size] ?? '')
</script>
