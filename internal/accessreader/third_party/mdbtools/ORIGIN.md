The files in this directory are from MDB Tools v1.0.1, published at
https://github.com/mdbtools/mdbtools/releases/tag/v1.0.1.

Source archive SHA-256:
ff9c425a88bc20bf9318a332eec50b17e77896eef65a0e69415ccb4e396d1812

Only libmdb source and required headers are included. The generated
`include/mdbtools.h` and `include/mdbver.h` were created by configuring that
release with `--disable-glib --disable-shared --enable-static` on Linux.
`COPYING.LIB` contains the upstream library license. Changes to these files
should be tracked against that upstream release.

Windows readiness modification: `include/mdbtools.h` selects the Windows CRT
`_locale_t` instead of the Linux-generated `locale_t` when `_WIN32` is defined.
Conformance follow-up modifications (2026-10-04):

- `include/mdbtools.h` adds a sticky `go_error` field to `MdbHandle`, consumed
  by the Go bridge. Error classes are defined in the adapter's `bridge.h`.
- `src/libmdb/file.c` records seek/read/short-page failures instead of padding
  incomplete pages and reporting success.
- `src/libmdb/data.c` records invalid row offsets/cracking errors and stops
  fetching after errors. Empty catalog slots and deleted-row skipping retain
  the upstream behavior.
- `src/libmdb/iconv.c` reports failed/incomplete/nonreversible conversions
  instead of inserting `?`, rejects incomplete compressed Unicode and lossy
  iconv options, and records unsupported encoding initialization.

Independent-review fixes (2026-10-04): `iconv.c` initializes conversion output
before malformed-input/allocation-failure returns and restricts output to
case-insensitive `UTF-8`/`UTF8` aliases, canonicalized to iconv's `UTF-8`.
`data.c` stops bound-column callers before string consumption on conversion/
allocation errors. Input `MDB_JET3_CHARSET` remains independently configurable.
The adapter's `tests/windows_conversion_test.c` fault injection is test-only;
the production allocator and LGPL source notices are unchanged.

Directly coupled embedded-NUL correction (2026-10-04): `iconv.c` checks the
produced UTF-8 byte span with `memchr` before appending its C-string terminator.
U+0000 source text produces the existing explicit sticky conversion error and
an empty terminated failure value, not a silently truncated prefix. Empty text
and the appended terminator remain valid. The native regression covers raw
conversion and actual bound-column callers for Jet3 CP1252/Jet4 UCS-2LE.

DATETIME correction (2026-10-05): `data.c` validates finite binary64 day
counts, eight-byte values, and the Access calendar range; extends the existing
calendar helper to signed integer days with absolute fractions; and supports
the adapter's explicit `%Y-%m-%d %H:%M:%S.%f` format. Fractional nanoseconds
are accepted only if they reconstruct the original source double. Ordinary
whole-second spellings are retained under the same check. Existing `strftime`
formats use their requested format (including short-date formats). Invalid
values retain the existing sticky row error. Synthetic tests modify disposable
fixture copies only; no Access runtime or source database writes are required.
The fractional suffix is formatted and canonicalized separately, then copied
only if its complete spelling and terminator fit the native bind buffer.
Formatting failure or insufficient capacity returns an explicit empty value
and sticky row error; bound callers also clear the output length.

The adapter's shared `libmdb.c` adds the Windows-only strict UTF-8 to UTF-16
read-only `_wfopen` shim, calling the bundled stream initializer. Linux still
uses upstream `mdb_open`. The reader/parser is not duplicated. Preserve this
modification record, upstream notices and `COPYING.LIB` with corresponding
source when evaluating redistribution and relinking obligations.

Description observation (2026-10-05): `include/mdbtools.h` and
`src/libmdb/props.c` retain ordered raw property entries (name, name-list index,
type/flag bytes, value bytes) alongside the unchanged compatibility hash and
retain each block's record type. The same parser checks chunk/name/entry
boundaries, native scalar-read minimum sizes, supported types and forward
progress before conversion; unsupported/malformed/truncated sequences free all
partial results and set the sticky property error. `src/libmdb/catalog.c`
records a missing full-OLE result as an error rather than silently omitting
metadata. Native type/flag codes are observations, not proven DAO NULL/type or
definition-origin semantics. Neither catalog links nor linked targets are
followed. Existing UTF-8/NUL, long-value and DATETIME patches are unchanged.
# Bounded native column observation addition

The reader-owned `mdb_read_columns_observed` reuses the existing native column
parser with opt-in checks for ordered observation completeness, supported type
codes, bounded counts, names and duplicate source numbers. `MdbColumn` retains
the original flag byte and size word before legacy normalization. Existing
`mdb_read_columns` keeps its compatibility path. No new Access binary parser,
schema-property interpretation, DDL or linked-target reader is introduced.
