// query prints card rows for the given ids from cards.cdb.
package main

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"

	_ "modernc.org/sqlite"
)

func main() {
	db, err := sql.Open("sqlite", "cards.cdb")
	if err != nil {
		panic(err)
	}
	defer db.Close()
	for _, a := range os.Args[1:] {
		id, _ := strconv.Atoi(a)
		var name, desc string
		var typ int
		row := db.QueryRow("SELECT name, type, desc FROM texts JOIN datas ON datas.id = texts.id WHERE datas.id = ?", id)
		if err := row.Scan(&name, &typ, &desc); err != nil {
			fmt.Println(a, "ERROR", err)
			continue
		}
		fmt.Println(a, "type", fmt.Sprintf("0x%x", typ), name)
	}
}
