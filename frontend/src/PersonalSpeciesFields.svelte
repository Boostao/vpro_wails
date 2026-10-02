<script lang="ts">
  import { personalLifeforms, personalTextFields, stagePersonalText, type PersonalSpeciesDraft } from './personalSpeciesEditor';
  let { draft, disabled = false, onchange }: {
    draft: PersonalSpeciesDraft; disabled?: boolean; onchange: (value: PersonalSpeciesDraft) => void;
  } = $props();
</script>

<div class="mb-3 grid grid-cols-1 gap-3 sm:grid-cols-2">
  <label class="flex flex-col gap-1" for="personal-code">Code
    <input id="personal-code" class="rounded border bg-white px-2 py-2" readonly value={draft.entered.toUpperCase()} />
  </label>
  <label class="flex flex-col gap-1" for="personal-lifeform">Lifeform
    <select id="personal-lifeform" class="rounded border bg-white px-2 py-2" value={draft.lifeform ?? ''}
      {disabled} onchange={event => onchange({ ...draft, lifeform: event.currentTarget.value === '' ? null : Number(event.currentTarget.value) })}>
      <option value="">NULL - not classified</option>
      {#each personalLifeforms as [value, label]}<option {value}>{label}</option>{/each}
    </select>
  </label>
</div>
{#each personalTextFields as {field, label} (field)}
  <div class="mb-3">
    <label class="flex flex-col gap-1" for={`personal-${field}`}>{label}
      <input id={`personal-${field}`} class="rounded border bg-white px-2 py-2" type="text" value={draft[field].raw}
        disabled={disabled || draft[field].isNull} aria-invalid={draft[field].error !== null}
        oninput={event => onchange(stagePersonalText(draft, field, event.currentTarget.value, false))} />
    </label>
    <label class="mt-1 flex items-center gap-2" for={`personal-null-${field}`}>
      <input id={`personal-null-${field}`} type="checkbox" checked={draft[field].isNull} {disabled}
        onchange={event => onchange(stagePersonalText(draft, field, draft[field].raw, event.currentTarget.checked))} />
      {label} - store NULL, not an empty string
    </label>
  </div>
{/each}
