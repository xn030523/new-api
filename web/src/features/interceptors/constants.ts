export const INTERCEPTOR_QUERY_KEY = ['interceptors'] as const
export const RULE_TYPES_QUERY_KEY = ['interceptor-rule-types'] as const

/** Custom rule types that need config fields */
export const CUSTOM_RULE_TYPES = [
  'custom_delete_path',
  'custom_set_value',
  'custom_delete_key_all',
  'custom_ensure_array',
  'custom_rename_key',
  'custom_reject_if',
] as const

/** Config field definitions for custom rule types */
export const CUSTOM_RULE_CONFIG_FIELDS: Record<
  string,
  Array<{ key: string; label: string; required: boolean; placeholder: string }>
> = {
  custom_delete_path: [
    {
      key: 'path',
      label: 'JSON Path',
      required: true,
      placeholder: 'e.g. metadata.user_id',
    },
  ],
  custom_set_value: [
    {
      key: 'path',
      label: 'JSON Path',
      required: true,
      placeholder: 'e.g. max_tokens',
    },
    {
      key: 'value',
      label: 'Value (JSON)',
      required: true,
      placeholder: 'e.g. 128000 or "string" or true',
    },
  ],
  custom_delete_key_all: [
    {
      key: 'key',
      label: 'Key Name',
      required: true,
      placeholder: 'e.g. annotations',
    },
  ],
  custom_ensure_array: [
    {
      key: 'path',
      label: 'JSON Path',
      required: true,
      placeholder: 'e.g. tools',
    },
  ],
  custom_rename_key: [
    {
      key: 'path',
      label: 'Parent Path',
      required: false,
      placeholder: 'e.g. thinking (empty for root)',
    },
    {
      key: 'old_key',
      label: 'Old Key',
      required: true,
      placeholder: 'e.g. enabled',
    },
    {
      key: 'new_key',
      label: 'New Key',
      required: true,
      placeholder: 'e.g. adaptive',
    },
  ],
  custom_reject_if: [
    {
      key: 'path',
      label: 'JSON Path',
      required: true,
      placeholder: 'e.g. max_tokens',
    },
    {
      key: 'op',
      label: 'Operator',
      required: true,
      placeholder: 'exists|not_exists|eq|ne|gt|lt|contains',
    },
    {
      key: 'value',
      label: 'Compare Value (JSON)',
      required: false,
      placeholder: 'e.g. 128000',
    },
    {
      key: 'message',
      label: 'Reject Message',
      required: false,
      placeholder: 'e.g. max_tokens too large',
    },
  ],
  cap_max_tokens: [
    {
      key: 'max',
      label: 'Max Value',
      required: false,
      placeholder: '128000',
    },
  ],
  reject_long_prompt: [
    {
      key: 'max_chars',
      label: 'Max Characters',
      required: false,
      placeholder: '1800000',
    },
  ],
}

export const INTERCEPTOR_PRESETS_QUERY_KEY = ['interceptor-presets'] as const
