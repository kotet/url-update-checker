package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/user"
	"runtime"
	"strconv"
	"time"

	"github.com/kotet/url-update-checker/internal/checker"
	"github.com/kotet/url-update-checker/internal/checker/entry"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	current, err := user.Current()
	if err != nil {
		log.Fatalln("[FATAL]", err)
	}
	databasepath := current.HomeDir + "/.config/url-update-watcher/db.sqlite"
	db, err := sql.Open("sqlite3", databasepath)
	if err != nil {
		log.Fatalln("[FATAL]", err)
	}
	_, err = db.Exec(
		`CREATE TABLE IF NOT EXISTS "URLS" ("ID" INTEGER PRIMARY KEY AUTOINCREMENT, "URL" TEXT, "MODIFIED" INTEGER, ETAG TEXT)`,
	)
	if err != nil {
		log.Fatalln("[FATAL]", err)
	}

	args := os.Args
	if len(args) < 2 {
		err = checker.Check(db)
		if err != nil {
			log.Fatalln("[FATAL]", err)
		}
		fmt.Println("Completed.")
		return
	}

	command := args[1]
	if command == "add" || command == "a" {
		if len(args) < 3 {
			log.Fatalf("usage: %v add <url>", args[0])
		}
		url := args[2]
		res, err := db.Exec(
			`INSERT INTO URLS (URL, MODIFIED, ETAG) VALUES (?,?,"")`,
			url, 0,
		)
		if err != nil {
			log.Fatalln("[FATAL]", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			log.Fatalln("[FATAL]", err)
		}
		fmt.Printf("Added: %v (id:%v)\n", url, id)
	}
	if command == "delete" || command == "r" || command == "d" {
		if len(args) < 3 {
			log.Fatalf("usage: %v add <id>", args[0])
		}
		id, err := strconv.Atoi(args[2])
		if err != nil {
			log.Fatalln("[FATAL]", err)
		}
		_, err = db.Exec(
			`DELETE FROM URLS WHERE ID=?`,
			id,
		)
		if err != nil {
			log.Fatalln("[FATAL]", err)
		}
		fmt.Printf("Deleted: %v\n", id)
	}
	if command == "list" || command == "l" {
		rows, err := db.Query(
			`SELECT * FROM URLS`,
		)
		if err != nil {
			log.Fatalln("[FATAL]", err)
		}
		defer rows.Close()
		fmt.Println("ID\tURL\tModified\tEtag")
		for rows.Next() {
			var e entry.Entry
			err = rows.Scan(&e.ID, &e.URL, &e.Modified, &e.Etag)
			if err != nil {
				log.Fatalln("[FATAL]", err)
			}
			fmt.Printf("%v\t%v\t%v\t%#v\n", e.ID, e.URL, time.Unix(e.Modified, 0), e.Etag)
		}
	}
	fmt.Printf("Usage: %v [add|delete|list]\n", args[0])
}
