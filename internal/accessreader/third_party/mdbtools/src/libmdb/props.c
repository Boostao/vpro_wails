/* MDB Tools - A library for reading MS Access database file
 * Copyright (C) 2000-2011 Brian Bruns and others
 *
 * This library is free software; you can redistribute it and/or
 * modify it under the terms of the GNU Library General Public
 * License as published by the Free Software Foundation; either
 * version 2 of the License, or (at your option) any later version.
 *
 * This library is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU
 * Library General Public License for more details.
 *
 * You should have received a copy of the GNU Library General Public
 * License along with this library; if not, write to the Free Software
 * Foundation, Inc., 51 Franklin Street, Fifth Floor, Boston, MA  02110-1301  USA
 */

#include "mdbtools.h"
#include <limits.h>

static void
props_error(MdbHandle *mdb)
{
	if (!mdb->go_error) mdb->go_error = GO_MDB_ERROR_PROPERTIES;
}

static GPtrArray *
mdb_read_props_list(MdbHandle *mdb, gchar *kkd, int len)
{
	guint32 record_len;
	int pos = 0;
	gchar *name;
	GPtrArray *names = NULL;
	int i=0;

	names = g_ptr_array_new();
#if MDB_DEBUG
	mdb_buffer_dump(kkd, 0, len);
#endif
	pos = 0;
	while (pos < len) {
		if (len - pos < 2) goto malformed;
		record_len = mdb_get_int16(kkd, pos);
		pos += 2;
		if (record_len > (guint32)(len - pos)) goto malformed;
		if (mdb_get_option(MDB_DEBUG_PROPS)) {
			fprintf(stderr, "%02d ",i++);
			mdb_buffer_dump(kkd, pos - 2, record_len + 2);
		}
		name = g_malloc(3*record_len + 1); /* worst case scenario is 3 bytes out per byte in */
		mdb_unicode2ascii(mdb, &kkd[pos], record_len, name, 3*record_len + 1);
		if (mdb->go_error) {
			g_free(name);
			goto malformed;
		}

		pos += record_len;
		g_ptr_array_add(names, name);
#if MDB_DEBUG
		printf("new len = %d\n", names->len);
#endif
	}
	return names;
malformed:
	props_error(mdb);
	for (guint j = 0; j < names->len; j++) g_free(g_ptr_array_index(names, j));
	g_ptr_array_free(names, TRUE);
	return NULL;
}
static void
free_hash_entry(gpointer key, gpointer value, gpointer user_data)
{
	g_free(key);
	g_free(value);
}
void
mdb_free_props(MdbProperties *props)
{
	if (!props) return;

	if (props->name) g_free(props->name);
	if (props->entries) {
		for (guint i = 0; i < props->entries->len; i++) {
			MdbPropertyEntry *entry = g_ptr_array_index(props->entries, i);
			g_free(entry->name);
			g_free(entry->bytes);
			g_free(entry);
		}
		g_ptr_array_free(props->entries, TRUE);
	}
	if (props->hash) {
		g_hash_table_foreach(props->hash, free_hash_entry, 0);
		g_hash_table_destroy(props->hash);
	}
	g_free(props);
}

static void
do_g_free(gpointer ptr, gpointer user_data) {
	g_free(ptr);
}

static void
free_names(GPtrArray *names) {
	g_ptr_array_foreach(names, do_g_free, NULL);
	g_ptr_array_free(names, TRUE);
}
MdbProperties *
mdb_alloc_props(void)
{
	MdbProperties *props;

	props = g_malloc0(sizeof(MdbProperties));

	return props;
}
static MdbProperties *
mdb_read_props(MdbHandle *mdb, GPtrArray *names, gchar *kkd, int len)
{
	guint32 record_len, name_len;
	int pos = 0;
	guint elem;
	int dtype, dsize;
	gchar *name;
	MdbProperties *props;
	int i=0;

#if MDB_DEBUG
	mdb_buffer_dump(kkd, 0, len);
#endif
	pos = 0;

	if (len < 6) {
		props_error(mdb);
		return NULL;
	}
	record_len = mdb_get_int32(kkd, pos);
	pos += 4;
	name_len = mdb_get_int16(kkd, pos);
	pos += 2;
	if (name_len > (guint32)(len - pos) || record_len != 6 + name_len) {
		props_error(mdb);
		return NULL;
	}
	props = mdb_alloc_props();
	if (name_len) {
		props->name = g_malloc(3*name_len + 1);
		mdb_unicode2ascii(mdb, kkd+pos, name_len, props->name, 3*name_len + 1);
		if (mdb->go_error) goto malformed;
		mdb_debug(MDB_DEBUG_PROPS,"prop block named: %s", props->name);
	}
	pos += name_len;

	props->hash = g_hash_table_new(g_str_hash, g_str_equal);
	props->entries = g_ptr_array_new();

	while (pos < len) {
		if (len - pos < 8) goto malformed;
		record_len = mdb_get_int16(kkd, pos);
		dtype = (unsigned char)kkd[pos + 3];
		elem = mdb_get_int16(kkd, pos + 4);
		if (elem >= names->len)
			goto malformed;
		dsize = mdb_get_int16(kkd, pos + 6);
		if (record_len != (guint32)(8 + dsize) || record_len > (guint32)(len - pos))
			goto malformed;
		switch (dtype) {
			case MDB_BOOL: case MDB_BYTE: if (dsize < 1) goto malformed; break;
			case MDB_INT: if (dsize < 2) goto malformed; break;
			case MDB_LONGINT: case MDB_FLOAT: if (dsize < 4) goto malformed; break;
			case MDB_MONEY: case MDB_DOUBLE: if (dsize < 8) goto malformed; break;
			case MDB_DATETIME: if (dsize != 8) goto malformed; break;
			case MDB_REPID: if (dsize < 16) goto malformed; break;
			case MDB_TEXT: case MDB_MEMO: case MDB_BINARY: case MDB_OLE: break;
			default: goto malformed;
		}
		name = g_ptr_array_index(names,elem);
		MdbPropertyEntry *entry = g_malloc0(sizeof(*entry));
		entry->name = g_strdup(name);
		entry->flags = (unsigned char)kkd[pos + 2];
		entry->type = dtype;
		entry->name_index = elem;
		entry->size = dsize;
		entry->bytes = g_malloc(dsize ? dsize : 1);
		if (dsize) memcpy(entry->bytes, kkd + pos + 8, dsize);
		g_ptr_array_add(props->entries, entry);
		if (mdb_get_option(MDB_DEBUG_PROPS)) {
			fprintf(stderr, "%02d ",i++);
			mdb_debug(MDB_DEBUG_PROPS,"elem %d (%s) dsize %d dtype %d", elem, name, dsize, dtype);
			mdb_buffer_dump(kkd + pos + 8, 0, dsize);
		}
		if (dtype == MDB_MEMO) {
			dtype = MDB_TEXT;
		} else if (dtype == MDB_BINARY && dsize == 16 && strcmp(name, "GUID") == 0) {
			dtype = MDB_REPID;
		}
		if (dtype == MDB_BOOL) {
			g_hash_table_insert(props->hash, g_strdup(name),
				g_strdup(kkd[pos + 8] ? "yes" : "no"));
		} else if (dtype == MDB_BINARY || dtype == MDB_OLE) {
			g_hash_table_insert(props->hash, g_strdup(name),
				g_strdup_printf("(binary data of length %d)", dsize));
		} else {
			g_hash_table_insert(props->hash, g_strdup(name),
			  mdb_col_to_string(mdb, kkd, pos + 8, dtype, dsize));
		}
		if (mdb->go_error) goto malformed;
		pos += record_len;
	}
	return props;
malformed:
	props_error(mdb);
	mdb_free_props(props);
	return NULL;
}

static void
print_keyvalue(gpointer key, gpointer value, gpointer outfile)
{
		fprintf((FILE*)outfile,"\t%s: %s\n", (gchar *)key, (gchar *)value);
}
void
mdb_dump_props(MdbProperties *props, FILE *outfile, int show_name) {
	if (show_name)
		fprintf(outfile,"name: %s\n", props->name ? props->name : "(none)");
	g_hash_table_foreach(props->hash, print_keyvalue, outfile);
	if (show_name)
		fputc('\n', outfile);
}

/*
 * That function takes a raw KKD/MR2 binary buffer,
 * typically read from LvProp in table MSysbjects
 * and returns a GPtrArray of MdbProps*
 */
GPtrArray*
mdb_kkd_to_props(MdbHandle *mdb, void *buffer, size_t len) {
	guint32 record_len;
	guint16 record_type;
	size_t pos;
	GPtrArray *names = NULL;
	MdbProperties *props;
	GPtrArray *result;

#if MDB_DEBUG
	mdb_buffer_dump(buffer, 0, len);
#endif
	if (!buffer || len < 4 || len > INT_MAX ||
			(memcmp("KKD\0", buffer, 4) && memcmp("MR2\0", buffer, 4))) {
		props_error(mdb);
		return NULL;
	}

	result = g_ptr_array_new();

	pos = 4;
	while (pos < len) {
		if (len - pos < 6) goto malformed;
		record_len = mdb_get_int32(buffer, pos);
		if (record_len < 6 || record_len > len - pos) goto malformed;
		record_type = mdb_get_int16(buffer, pos + 4);
		mdb_debug(MDB_DEBUG_PROPS,"prop chunk type:0x%04x len:%d", record_type, record_len);
		//mdb_buffer_dump(buffer, pos+4, record_len);
		switch (record_type) {
			case 0x80:
				if (names) free_names(names);
				names = mdb_read_props_list(mdb, (char*)buffer+pos+6, record_len - 6);
				if (!names) goto malformed;
				break;
			case 0x00:
			case 0x01:
			case 0x02:
				if (!names) {
					goto malformed;
				}
				props = mdb_read_props(mdb, names, (char*)buffer+pos+6, record_len - 6);
				if (!props) goto malformed;
				props->record_type = record_type;
				g_ptr_array_add(result, props);
				//mdb_dump_props(props, stderr, 1);
				break;
			default:
				goto malformed;
		}
		pos += record_len;
	}
	if (names) free_names(names);
	return result;
malformed:
	props_error(mdb);
	if (names) free_names(names);
	for (guint i = 0; i < result->len; i++) mdb_free_props(g_ptr_array_index(result, i));
	g_ptr_array_free(result, TRUE);
	return NULL;
}
