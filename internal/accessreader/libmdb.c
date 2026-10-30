//go:build linux || windows

#include "bridge.h"
#ifdef _WIN32
#include <windows.h>
#include <winnls.h>
#ifndef WC_ERR_INVALID_CHARS
#define WC_ERR_INVALID_CHARS 0x00000080
#endif
#endif

#include "third_party/mdbtools/src/libmdb/catalog.c"
#include "third_party/mdbtools/src/libmdb/file.c"
#include "third_party/mdbtools/src/libmdb/table.c"
#include "third_party/mdbtools/src/libmdb/data.c"
#include "third_party/mdbtools/src/libmdb/dump.c"
#include "third_party/mdbtools/src/libmdb/backend.c"
#include "third_party/mdbtools/src/libmdb/money.c"
#include "third_party/mdbtools/src/libmdb/sargs.c"
#include "third_party/mdbtools/src/libmdb/index.c"
#include "third_party/mdbtools/src/libmdb/like.c"
#include "third_party/mdbtools/src/libmdb/write.c"
#include "third_party/mdbtools/src/libmdb/stats.c"
#include "third_party/mdbtools/src/libmdb/map.c"
#include "third_party/mdbtools/src/libmdb/props.c"
#include "third_party/mdbtools/src/libmdb/worktable.c"
#include "third_party/mdbtools/src/libmdb/options.c"
#include "third_party/mdbtools/src/libmdb/iconv.c"
#include "third_party/mdbtools/src/libmdb/version.c"
#include "third_party/mdbtools/src/libmdb/rc4.c"
#include "third_party/mdbtools/src/libmdb/fakeglib.c"

MdbHandle *go_mdb_open(const char *path, int *error) {
    *error = GO_MDB_ERROR_NONE;
#ifdef _WIN32
    int count = MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, path, -1, NULL, 0);
    if (!count) return NULL;
    wchar_t *wide = calloc(count, sizeof(*wide));
    if (!wide) return NULL;
    if (!MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, path, -1, wide, count)) {
        free(wide);
        return NULL;
    }
    FILE *stream = _wfopen(wide, L"rb");
    free(wide);
    if (!stream) return NULL;
    if (fseeko(stream, 0, SEEK_END) != 0 || ftello(stream) < 2048 ||
            fseeko(stream, 0, SEEK_SET) != 0) {
        *error = GO_MDB_ERROR_READ;
        fclose(stream);
        return NULL;
    }
    MdbHandle *mdb = mdb_handle_from_stream(stream, MDB_NOFLAGS);
#else
    MdbHandle *mdb = mdb_open(path, MDB_NOFLAGS);
#endif
    if (!mdb) return NULL;
    if (fseeko(mdb->f->stream, 0, SEEK_END) != 0 ||
            ftello(mdb->f->stream) % mdb->fmt->pg_size != 0) {
        if (!mdb->go_error) mdb->go_error = GO_MDB_ERROR_READ;
    }
    if (mdb->go_error) {
        *error = mdb->go_error;
        mdb_close(mdb);
        return NULL;
    }
    return mdb;
}