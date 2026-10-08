# Long Vegetation workbook draft

## Availability

The private generator implements unit worksheets and the source ReportSummary
worksheet. The independently reviewed owned publisher has a registered public
DTO facade and interface bindings, default-off under
`VPRO_LONG_VEGETATION_WORKBOOK`, paired with literal frontend
`VITE_LONG_VEGETATION_WORKBOOK`. The existing Long Vegetation route's
`VITE_LONG_VEGETATION_REPORT` gate is still required; Code/Strata/Lifeform gates
remain independent when those retained settings are selected. This bounded
unlumped desktop workflow passes native and integration checks; it does not
establish all Access report modes. Existing preview behavior is unchanged.

Source: `VPro64_forAI/Modules/V7mdlReportsLongVeg.txt`, `LongVegReport`,
`AddLifeformCodes` and `LongVegOptionText`; launch preferences/events are in
`Forms/USysLongVegOptions.txt`. AllSpecs version comes from the table Description
property, mapped to SQLite `_table_metadata`, not species rows or an R version.

## Unit sheets

- Title, unit/unique long name and unlumped space occupy rows 1-3.
- Row 4 contains group, species, optional English Name/Code, presence, mean cover
  and sorted pivot plot headings. Observations begin at row 5.
- Preserve typed numeric results, NULL blanks, original row order and unit
  identity. The source's 250-plot per-unit limit remains.
- Group-gap scanning begins at worksheet row 7, stops at the next blank group,
  and scans at most 1,000 transitions. Rows 5/6 do not receive a gap.
- Zero-count units retain explicit skipped-unit evidence. A nonempty unit with
  no observation/pivot evidence is refused, not replaced by guessed headings.
- Non-quick unit sheets retain source landscape printing, .75-inch side/bottom
  margins, .5-inch top margin and page/date footer. Freeze panes and useful
  widths are desktop presentation adaptations.
- Formula-shaped text stays literal. Reuse Environment workbook XML/text,
  UTF-16 bounds and source sheet-name validation. Refuse collisions rather than
  renaming or overwriting sheets.

## ReportSummary

`LVReportSummary` defaults to true in the source. A requested summary requires
captured summary evidence; it must never be silently omitted.

The private summary builder requires a reviewed calendar date, complete physical
Env/SU rows and Description definitions specifically owned by `USysAllSpecs`.
It preserves:

- Source labels and rows 4-13, grouping/order captions and numeric thresholds.
  Presence uses the source's divided-by-100 ratio in B9, not the stored
  percentage preference (50 becomes numeric 0.5).
- Original Env record count, including NULL plots and duplicates. The source
  labels this "Number of plots in database" and places the Env table name under
  "Vegetation table:"; these labels are not silently corrected.
- Count of all non-NULL SU PlotNumber rows, including empty text and duplicates,
  when quality filtering is off. This is not a distinct-plot count.
- Every SU membership at D:E, in captured row order, including NULL/empty cells
  and physical row identities in hidden provenance.
- Literal Description text, including empty text; NULL stays NULL/visibly blank.
  Absent metadata and absent definitions both display the source error fallback
  `Unknown`, but have different recorded states. Duplicate definitions and
  malformed/unsupported metadata refuse output; no arbitrary first match.
- The summary sheet follows unit sheets. Case-insensitive unit name collisions
  with `ReportSummary` refuse the complete workbook. Compare with
  `strings.EqualFold`, matching Excelize's worksheet resolution: lowercasing
  alone misses names such as `S` versus the Unicode long s.

Intentional presentation adaptations:

1. The source concatenates locale-dependent `Date`. The draft requires an
   explicit `YYYY-MM-DD` date (years 0100-9999). The future owned review must
   capture that date and reuse it at publication, including across midnight.
   It is not the fixed 2000-01-01 document timestamp.
2. The source writes Lifeform legend E:F, then overwrites its codes with SU
   memberships D:E. The draft preserves the exact 0-13 descriptions at G:H,
   separate from memberships, instead of reproducing this destructive overlap.
3. Source QCLV replaces the selected SU with temporary `USysDeleteMe_SU`.
   For enforced quality, summary count/memberships use the accepted report's
   filtered join occurrences, retaining duplicate weights and complete quality
   provenance. Repeated memberships keep their original physical IDs, never
   invented temporary row IDs. The visible table caption remains the original
   selected physical table, not a phantom temporary table; no tables or registry
   selection are created/changed.

The generator currently accepts only the existing unlumped preview. Its summary
therefore explicitly displays `None_Lump`; this does not infer the unmapped
Access `CurrLump` selection. Public review must require an explicit supported
unlumped scope and refuse combine-variants/lumping until implemented.

## Provenance and outstanding acceptance

Very-hidden `_VPRO_Source` stores chunked hex of the full typed preview and
layout, including reviewed date, raw Description definitions, counts and typed
memberships. Identical captured inputs produce identical bytes. All errors,
cancellation and workbook cleanup failures return no successful partial bytes.

The private publisher requires explicit `unlumped` scope and a literal default-off
`VPRO_LONG_VEGETATION_WORKBOOK` constructor gate. It captures fresh strict YAML
options/layout and complete relevant physical tables within owned context/read
snapshots. Approval covers the whole parsed configuration, raw captured tables,
scope, reviewed creation date and workbook bytes. Canonical parsed YAML authority
is intentional: comments/formatting do not invalidate review, any parsed preference
change does, including an otherwise-unreported key.

Export reuses the reviewed date instead of consulting the clock again. Fresh
initial/prelink/postlink source checks use the shared no-replace artifact publisher.
Known outcomes are `published`, `published-with-errors` and `not-published`;
published errors explicitly forbid replay. Registration remains default-off.
Frontend shared publication sessions retain unknown receipts through tab remount
and explicit acknowledgement, invalidate old preparation and require fresh review.
Unresolved irreversible calls block navigation, native close and overlapping reads;
only source/review reads are cancellable.

## Bounded acceptance

- Complete frontend580 tests, Svelte check0 errors/warnings and isolated enabled/
  workbook-off frontend/production Wails builds.
- Independent kernel/shared-source, owned-publisher and final desktop reviews;
  resolved Unicode-fold sheet collision and exact positive threshold regression.
- Twelve native cases/four normally closed owners: actual1400/600px labels and
  containment, reviewed publication/collision, held irreversible known/lost
  receipts, navigation/native-close/read overlap, remount/acknowledgement,
  independent gates and held read cancellation/stale suppression/explicit retry.
- Two actual98167-byte outputs each independently match reviewed hash/typed XML,
  eleven unit sheets,869 observation rows and ReportSummary.
- Every original disposable data/config/audit identity restores after closure;
  no database seeds, source temporary tables or audit writes. The first
  zero-output candidate was normally closed/hash-archived before replacing it
  with the correctly gated retained Strata/Code candidate.
- All-package race passes with fresh root1508.347s and680 unchanged frozen source
  identities. The immutable archive seals the exact integration evidence.

Source4 attributes, lumping, combined variants and empty pivot units remain
unavailable; no parity claim for those modes. The full forms/reports migration is
not complete.
