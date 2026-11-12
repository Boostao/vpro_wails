#ifndef GO_MDBTOOLS_BRIDGE_H
#define GO_MDBTOOLS_BRIDGE_H

#include <mdbtools.h>

enum {
    GO_MDB_ERROR_NONE = 0,
    GO_MDB_ERROR_READ = 1,
    GO_MDB_ERROR_ENCODING = 2,
    GO_MDB_ERROR_TEXT = 3,
    GO_MDB_ERROR_ROW = 4,
    GO_MDB_ERROR_PROPERTIES = 5,
    GO_MDB_ERROR_SCHEMA = 6
};

MdbHandle *go_mdb_open(const char *path, int *error);

typedef struct {
    MdbHandle *mdb;
    MdbTableDef *table;
    char **values;
    int *lengths;
    unsigned char *nulls;
} GoMdbReader;

GoMdbReader *go_mdb_open_reader(const char *path, const char *table_name, int *error);
void go_mdb_close_reader(GoMdbReader *reader);
int go_mdb_next(GoMdbReader *reader);
void go_mdb_clear_row_error(GoMdbReader *reader);
MdbColumn *go_mdb_column(GoMdbReader *reader, int index);
MdbCatalogEntry *go_mdb_entry(GPtrArray *catalog, unsigned int index);
MdbProperties *go_mdb_property_block(MdbCatalogEntry *entry, unsigned int index);
MdbPropertyEntry *go_mdb_property_entry(MdbProperties *block, unsigned int index);
MdbColumn *go_mdb_schema_column(MdbTableDef *table, unsigned int index);
int go_mdb_is_null(GoMdbReader *reader, int index);
int go_mdb_memo_too_large(GoMdbReader *reader, int index);
int go_mdb_length(GoMdbReader *reader, int index);
const char *go_mdb_text(GoMdbReader *reader, int index);
void *go_mdb_blob(GoMdbReader *reader, int index, size_t *size);

#endif