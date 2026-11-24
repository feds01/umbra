package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/feds01/umbra"
)

// relations lists each column of every table, partitioned table, materialized view and
// foreign table outside the system schemas and extensions, with a domain resolved to its
// base type and every enum written "enum". A partition is left to its parent.
const relations = `
WITH RECURSIVE domain_base(domain_oid, base_oid, base_mod) AS (
	SELECT t.oid, t.typbasetype, t.typtypmod FROM pg_type t WHERE t.typtype = 'd'
	UNION ALL
	SELECT d.domain_oid, t.typbasetype, t.typtypmod
	FROM domain_base d JOIN pg_type t ON t.oid = d.base_oid AND t.typtype = 'd'
)
SELECT n.nspname, c.relname, a.attname,
	CASE WHEN resolved.typtype = 'e' THEN 'enum' ELSE
		format_type(COALESCE(b.base_oid, a.atttypid), CASE WHEN b.base_oid IS NULL THEN a.atttypmod ELSE b.base_mod END)
	END
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
JOIN pg_attribute a ON a.attrelid = c.oid
LEFT JOIN domain_base b ON b.domain_oid = a.atttypid
	AND NOT EXISTS (SELECT 1 FROM pg_type bt WHERE bt.oid = b.base_oid AND bt.typtype = 'd')
JOIN pg_type resolved ON resolved.oid = COALESCE(b.base_oid, a.atttypid)
WHERE c.relkind IN ('r', 'p', 'm', 'f')
	AND NOT c.relispartition
	AND a.attnum > 0
	AND NOT a.attisdropped
	AND n.nspname NOT IN ('pg_catalog', 'information_schema')
	AND n.nspname NOT LIKE 'pg\_toast%'
	AND n.nspname NOT LIKE 'pg\_temp%'
	AND NOT EXISTS (
		SELECT 1 FROM pg_depend d WHERE d.classid = 'pg_class'::regclass AND d.objid = c.oid AND d.deptype = 'e'
	)`

// ReadSchema reads the columns of every table, partitioned table, materialized view and
// foreign table in the database, outside the system schemas and extensions. A table in
// schema is named bare and any other schema.table. A domain's type is its base type.
func ReadSchema(ctx context.Context, db *sql.DB, schema string) (umbra.Schema, error) {
	rows, err := db.QueryContext(ctx, relations)
	if err != nil {
		return nil, fmt.Errorf("read the schema: %w", err)
	}

	defer rows.Close()

	tables := umbra.Schema{}

	for rows.Next() {
		var namespace, table, column, typeName string

		if err := rows.Scan(&namespace, &table, &column, &typeName); err != nil {
			return nil, fmt.Errorf("read the schema: %w", err)
		}

		if namespace != schema {
			table = namespace + "." + table
		}

		if tables[table] == nil {
			tables[table] = map[string]umbra.Type{}
		}

		tables[table][column] = typeOf(typeName)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read the schema: %w", err)
	}

	return tables, nil
}

// typeOf sorts a type, as format_type writes it, by the kind of value it holds. A type it
// doesn't know, such as jsonb, bytea or a composite, is Opaque.
func typeOf(name string) umbra.Type {
	element, isArray := strings.CutSuffix(name, "[]")

	return umbra.Type{Name: name, Kind: kindOf(withoutModifier(element)), Array: isArray}
}

func kindOf(name string) umbra.Kind {
	switch name {
	case "text", "citext", "character varying", "character":
		return umbra.Text
	case "uuid":
		return umbra.UUID
	case "inet":
		return umbra.Inet
	case "enum", "boolean", "smallint", "integer", "bigint", "real", "double precision", "numeric",
		"date", "time without time zone", "time with time zone",
		"timestamp without time zone", "timestamp with time zone", "interval":
		return umbra.Scalar
	}

	return umbra.Opaque
}

// withoutModifier drops a type's modifier: character varying(255) is character varying,
// and timestamp(3) with time zone is timestamp with time zone.
func withoutModifier(name string) string {
	before, rest, hasModifier := strings.Cut(name, "(")
	if !hasModifier {
		return name
	}

	_, after, _ := strings.Cut(rest, ")")

	return before + after
}
