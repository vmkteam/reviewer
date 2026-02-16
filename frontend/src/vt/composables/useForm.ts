import { ref, reactive } from 'vue'
import type { FieldError } from '../../api/vt'
import { RpcError } from '../../api/vtClient'

interface FormApi<T> {
  getByID: (id: number) => Promise<T>
  add: (entity: Partial<T>) => Promise<T>
  update: (entity: Partial<T>) => Promise<boolean>
  delete: (id: number) => Promise<boolean>
  validate: (entity: Partial<T>) => Promise<FieldError[]>
}

export function useForm<T extends { id: number }>(api: FormApi<T>, defaults: () => Partial<T>) {
  const entity = reactive<Partial<T>>(defaults()) as T
  const errors = ref<FieldError[]>([])
  const loading = ref(false)
  const saving = ref(false)
  const error = ref('')

  function fieldError(field: string): string {
    const fe = errors.value.find(e => e.field === field)
    if (!fe) return ''
    switch (fe.error) {
      case 'required': return 'Required field'
      case 'max': return `Maximum ${fe.constraint?.max} characters`
      case 'min': return `Minimum ${fe.constraint?.min} characters`
      case 'incorrect': return 'Incorrect value'
      case 'unique': return 'Value must be unique'
      case 'format': return 'Invalid format'
      default: return fe.error
    }
  }

  async function load(id: number) {
    loading.value = true
    error.value = ''
    try {
      const data = await api.getByID(id)
      Object.assign(entity, data)
    } catch (e: unknown) {
      error.value = e instanceof Error ? e.message : 'Unknown error'
    } finally {
      loading.value = false
    }
  }

  async function save(): Promise<boolean> {
    saving.value = true
    errors.value = []
    error.value = ''
    try {
      const validationErrors = await api.validate(entity)
      if (validationErrors && validationErrors.length > 0) {
        errors.value = validationErrors
        return false
      }

      if (entity.id) {
        await api.update(entity)
      } else {
        const created = await api.add(entity)
        Object.assign(entity, created)
      }
      return true
    } catch (e: unknown) {
      if (e instanceof RpcError && e.data) {
        const data = e.data as FieldError[]
        if (Array.isArray(data)) {
          errors.value = data
          return false
        }
      }
      error.value = e instanceof Error ? e.message : 'Unknown error'
      return false
    } finally {
      saving.value = false
    }
  }

  async function remove(id: number): Promise<boolean> {
    try {
      return await api.delete(id)
    } catch (e: unknown) {
      error.value = e instanceof Error ? e.message : 'Unknown error'
      return false
    }
  }

  function reset() {
    Object.assign(entity, defaults())
    errors.value = []
    error.value = ''
  }

  return {
    entity,
    errors,
    loading,
    saving,
    error,
    fieldError,
    load,
    save,
    remove,
    reset,
  }
}
