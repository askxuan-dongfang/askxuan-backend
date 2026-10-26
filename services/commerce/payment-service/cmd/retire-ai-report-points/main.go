package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/askxuan/payment-service/internal/reportcleanup"
	"github.com/go-sql-driver/mysql"
	"os"
	"time"
)

var sourceRevision = "development"

const approvedBatch = "ai-report-wallet-20261026"

var approvedCounts = reportcleanup.Expected{Reports: 35, Ledgers: 43, Points: 430, Sessions: 2, Messages: 6}

func main() {
	version := flag.Bool("version", false, "print build source revision without accessing a database")
	mode := flag.String("mode", "plan", "plan, apply or rollback")
	manifest := flag.String("manifest", "", "reviewed manifest path")
	backup := flag.String("backup", "", "private backup JSON path (mode 0600)")
	flag.Parse()
	if *version {
		fmt.Println(sourceRevision)
		return
	}
	fail := func() {
		fmt.Fprintln(os.Stderr, "retirement failed: review snapshot guards and operator logs; no partial transaction is committed")
		os.Exit(1)
	}
	if *manifest == "" || *backup == "" || os.Getenv("AI_REPORT_CLEANUP_DSN") == "" {
		fail()
	}
	config, e := mysql.ParseDSN(os.Getenv("AI_REPORT_CLEANUP_DSN"))
	if e != nil {
		fail()
	}
	if config.Params == nil {
		config.Params = map[string]string{}
	}
	config.Params["charset"] = "utf8mb4"
	config.ParseTime = false // Preserve MySQL timestamp strings for exact restoration.
	db, e := sql.Open("mysql", config.FormatDSN())
	if e != nil {
		fail()
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	var b reportcleanup.Backup
	var m reportcleanup.Manifest
	if *mode == "plan" {
		b, m, e = reportcleanup.Plan(ctx, db, approvedBatch, approvedCounts)
		if e != nil {
			fail()
		}
		// Exclusive creation prevents replacing the only usable rollback backup.
		for name, value := range map[string]any{*backup: b, *manifest: m} {
			raw, err := json.MarshalIndent(value, "", "  ")
			if err != nil {
				fail()
			}
			f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
			if err != nil {
				fail()
			}
			_, err = f.Write(append(raw, '\n'))
			if err == nil {
				err = f.Sync()
			}
			closeErr := f.Close()
			if err != nil || closeErr != nil {
				fail()
			}
		}
	} else {
		raw, err := os.ReadFile(*backup)
		if err != nil || json.Unmarshal(raw, &b) != nil {
			fail()
		}
		raw, err = os.ReadFile(*manifest)
		if err != nil || json.Unmarshal(raw, &m) != nil {
			fail()
		}
		if b.Batch != approvedBatch || m.Batch != approvedBatch || m.Expected != approvedCounts {
			fail()
		}
		switch *mode {
		case "apply":
			e = reportcleanup.Apply(ctx, db, b, m)
		case "rollback":
			e = reportcleanup.Rollback(ctx, db, b, m)
		default:
			fail()
		}
		if e != nil {
			fail()
		}
	}
	result := map[string]any{"mode": *mode, "batch": m.Batch, "manifestSha256": m.SHA256, "counts": m.Expected, "preservedUsageRows": m.PreservedUsageRows, "sourceRevision": sourceRevision}
	raw, _ := json.Marshal(result)
	fmt.Println(string(raw))
}
