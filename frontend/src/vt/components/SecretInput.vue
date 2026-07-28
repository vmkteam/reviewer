<template>
  <div class="relative">
    <VInput
      class="pr-10"
      :type="revealed ? 'text' : 'password'"
      autocomplete="new-password"
      :model-value="modelValue"
      :placeholder="savedMasked ? 'Saved — leave blank to keep' : placeholder"
      :readonly="readonly"
      v-bind="$attrs"
      @update:model-value="emit('update:modelValue', $event)"
    />
    <button
      type="button"
      @click="revealed = !revealed"
      class="absolute inset-y-0 right-0 flex items-center px-3 text-fg-subtle hover:text-fg transition-colors"
      :title="revealed ? 'Hide' : 'Show'"
      tabindex="-1"
    >
      <svg v-if="revealed" xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M3.707 2.293a1 1 0 00-1.414 1.414l14 14a1 1 0 001.414-1.414l-1.473-1.473A10.014 10.014 0 0019.542 10C18.268 5.943 14.478 3 10 3a9.958 9.958 0 00-4.512 1.074l-1.78-1.781zm4.261 4.26l1.514 1.515a2.003 2.003 0 012.45 2.45l1.514 1.514a4 4 0 00-5.478-5.478z" clip-rule="evenodd" /><path d="M12.454 16.697L9.75 13.992a4 4 0 01-3.742-3.741L2.335 6.578A9.98 9.98 0 00.458 10c1.274 4.057 5.065 7 9.542 7 .847 0 1.669-.105 2.454-.303z" /></svg>
      <svg v-else xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" viewBox="0 0 20 20" fill="currentColor"><path d="M10 12a2 2 0 100-4 2 2 0 000 4z" /><path fill-rule="evenodd" d="M.458 10C1.732 5.943 5.522 3 10 3s8.268 2.943 9.542 7c-1.274 4.057-5.064 7-9.542 7S1.732 14.057.458 10zM14 10a4 4 0 11-8 0 4 4 0 018 0z" clip-rule="evenodd" /></svg>
    </button>
  </div>
  <p v-if="savedMasked" class="mt-1 text-xs text-fg-subtle font-mono">Saved: {{ savedMasked }}</p>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import VInput from './VInput.vue'

defineOptions({ inheritAttrs: false })

defineProps<{
  modelValue?: unknown
  placeholder?: string
  readonly?: boolean
  // Masked form of the stored secret. When set, the input advertises
  // set-or-keep semantics ("Saved — leave blank to keep") and shows the hint.
  savedMasked?: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const revealed = ref(false)
</script>
