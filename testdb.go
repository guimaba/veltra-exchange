package main

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	credentials := []string{
		"root:@tcp(127.0.0.1:3306)/",
		"root:root@tcp(127.0.0.1:3306)/",
		"admin:admin@tcp(127.0.0.1:3306)/",
	}

	for _, dsn := range credentials {
		db, err := sql.Open("mysql", dsn)
		if err == nil {
			err = db.Ping()
			if err == nil {
				fmt.Printf("SUCCESS: %s\n", dsn)
				os.Exit(0)
			}
		}
	}
	fmt.Println("FAILED to connect with common credentials")
}
