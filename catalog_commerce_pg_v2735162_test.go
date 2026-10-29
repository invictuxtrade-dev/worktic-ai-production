package main

import (
	"strings"
	"testing"
)

func TestCatalogSaleEvidenceDDLPostgresV2735162(t *testing.T) {
	ddl := catalogSaleEvidenceDDLV2735162(true)
	if !strings.Contains(ddl, "content BYTEA NOT NULL") {
		t.Fatalf("PostgreSQL requires BYTEA; DDL: %s", ddl)
	}
	if strings.Contains(ddl, "content BLOB") {
		t.Fatal("PostgreSQL DDL contains SQLite BLOB")
	}
}

func TestCatalogSaleEvidenceDDLSQLiteV2735162(t *testing.T) {
	ddl := catalogSaleEvidenceDDLV2735162(false)
	if !strings.Contains(ddl, "content BLOB NOT NULL") {
		t.Fatalf("SQLite requires BLOB; DDL: %s", ddl)
	}
}
