export interface EnterPosition {
  form: string | null;
  record: string | null;
  column: string;
  enabled: boolean;
  visible: boolean;
  invalid: boolean;
}

export const sourceNextRecordForms: ReadonlySet<string> = new Set([
  'SubVegAXL_BC', 'SubVegCXL', 'SubVegDXL', 'SoilHumusXL', 'SoilMineralXL',
]);

export function enterNavigationMode(form: string | null): 'field' | 'record' | null {
  return form === null ? 'field' : sourceNextRecordForms.has(form) ? 'record' : null;
}

export function nextEnterPosition(positions: readonly EnterPosition[], current: number, mode: 'field' | 'record'): number | null {
  if (!Number.isInteger(current) || current < 0 || current >= positions.length) {
    throw new Error('Enter navigation requires the currently observed source control.');
  }
  const source = positions[current];
  if (!source.column || mode === 'record' && (source.form === null || source.record === null)) {
    throw new Error('Enter navigation requires an observed literal column and existing physical record.');
  }
  if (!source.enabled || !source.visible || source.invalid) return null;
  for (let index = current + 1; index < positions.length; index++) {
    const candidate = positions[index];
    if (!candidate.enabled || !candidate.visible) continue;
    if (mode === 'field' || candidate.form === source.form && candidate.record !== null &&
        candidate.record !== source.record && candidate.column === source.column) return index;
  }
  return null;
}

function entryControl(element: Element): element is HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement {
  return element instanceof HTMLSelectElement || element instanceof HTMLTextAreaElement || element instanceof HTMLInputElement &&
    ['text', 'number', 'date', 'time', 'datetime-local'].includes(element.type);
}

export function navigateSourceEnter(event: KeyboardEvent, root: HTMLElement): void {
  const source = event.target;
  if (event.key !== 'Enter' || event.defaultPrevented || event.isComposing || event.repeat ||
      event.altKey || event.ctrlKey || event.metaKey || event.shiftKey || !(source instanceof HTMLInputElement) ||
      !entryControl(source) || source.list || !root.contains(source) || document.activeElement !== source ||
      source.closest('dialog,[role=dialog]')) return;
  const child = source.closest<HTMLElement>('[data-source-form]');
  const area = child ?? source.closest<HTMLElement>('[data-source-page]');
  if (!area) return;
  const mode = enterNavigationMode(child?.dataset.sourceForm ?? null);
  if (!mode) return;
  const controls = Array.from(area.querySelectorAll('input[data-column],select[data-column],textarea[data-column]')).filter(entryControl);
  const current = controls.indexOf(source);
  if (current < 0) return;
  const positions = controls.map(control => ({
    form: control.closest<HTMLElement>('[data-source-form]')?.dataset.sourceForm ?? null,
    record: control.closest<HTMLElement>('[data-record-id]')?.dataset.recordId ?? null,
    column: control.dataset.column ?? '',
    enabled: !control.matches(':disabled') && !((control instanceof HTMLInputElement || control instanceof HTMLTextAreaElement) && control.readOnly),
    visible: control.checkVisibility(),
    invalid: control.getAttribute('aria-invalid') === 'true',
  }));
  event.preventDefault();
  const next = nextEnterPosition(positions, current, mode);
  if (next === null) return;
  const destination = controls[next];
  destination.focus();
  if (document.activeElement !== destination) {
    throw new Error('The observed next source control could not receive focus; no record was saved or created.');
  }
}
