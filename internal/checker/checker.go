package checker

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/user"
	"strconv"
	"sync"
	"time"

	"github.com/gosuri/uiprogress"
	"github.com/kotet/url-update-checker/internal/checker/entry"
	_ "github.com/mattn/go-sqlite3"
)

func Check(db *sql.DB) error {

	entries, err := getEntries(db)
	if err != nil {
		return err
	}

	n, err := count(db)
	if err != nil {
		return err
	}

	if n == 0 {
		fmt.Println("No entry")
		os.Exit(1)
	}

	progress := uiprogress.New()
	bar := progress.AddBar(n).AppendCompleted().PrependFunc(func(b *uiprogress.Bar) string {
		return fmt.Sprintf("%v/%v", b.Current(), n)
	})
	progress.Start()
	var wait sync.WaitGroup
	for _, entry := range entries {
		wait.Add(1)
		go func(id uint64, url string, modified int64, etag string) {
			defer wait.Done()
			res, err := http.Get(url)
			if err != nil {
				fmt.Fprintln(progress.Bypass(), "[Error]", err)
				return
			}
			defer res.Body.Close()
			newModifiedText := res.Header.Get("Last-Modified")
			if newModifiedText == "" {
				fmt.Fprintln(progress.Bypass(), "[Warn]", url, " has no Last-Modified header. Etag is used instead.")
				newEtag := res.Header.Get("Etag")
				if newEtag == "" {
					fmt.Fprintln(progress.Bypass(), "[Error]", url, "has no Last-Modified and Etag header.")
					return
				}
				if etag != newEtag {
					_, err := db.Exec(
						`UPDATE URLS SET ETAG=? WHERE ID=?`,
						newEtag, id,
					)
					if err != nil {
						fmt.Fprintln(progress.Bypass(), "[Error]", err)
						return
					}
					err = savePage(*res, id)
					if err != nil {
						fmt.Fprintln(progress.Bypass(), "[Error]", err)
						return
					}
					fmt.Fprintln(progress.Bypass(), "Etag changed: ", url, " (", etag, " -> ", newEtag, ")")
				}
			}
			newModified, err := time.Parse(http.TimeFormat, newModifiedText)
			if modified < newModified.Unix() {
				_, err = db.Exec(
					`UPDATE URLS SET MODIFIED=? WHERE ID=?`,
					newModified.Unix(), id,
				)
				if err != nil {
					fmt.Fprintln(progress.Bypass(), "[Error]", err)
					return
				}
				err = savePage(*res, id)
				if err != nil {
					fmt.Fprintln(progress.Bypass(), "[Error]", err)
					return
				}
				fmt.Fprintln(progress.Bypass(), "Last-Modified changed: ", url, " (", time.Unix(modified, 0), " -> ", newModified, ")")
			}
			bar.Incr()
		}(entry.ID, entry.URL, entry.Modified, entry.Etag)
	}
	wait.Wait()
	progress.Stop()
	return nil
}

func getEntries(db *sql.DB) ([]entry.Entry, error) {
	rows, err := db.Query(
		`SELECT * FROM URLS`,
	)
	if err != nil {
		return nil, err
	}
	var entries []entry.Entry
	defer rows.Close()
	for rows.Next() {
		var e entry.Entry
		err = rows.Scan(&e.ID, &e.URL, &e.Modified, &e.Etag)
		if err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func count(db *sql.DB) (int, error) {
	count, err := db.Query(
		`SELECT COUNT(ID) FROM URLS`,
	)
	if err != nil {
		return 0, err
	}
	defer count.Close()
	var n int
	count.Next()
	err = count.Scan(&n)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func savePage(response http.Response, id uint64) error {
	current, err := user.Current()
	if err != nil {
		return err
	}
	pagedir := current.HomeDir + "/.cache/url-update-checker/" + strconv.FormatUint(id, 10)
	pagepath := pagedir + "/" + time.Now().Format("2006-01-02T150405Z0700")
	os.MkdirAll(pagedir, os.ModePerm)
	file, err := os.OpenFile(pagepath, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	defer file.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}
	file.Write(body)
	return nil
}
