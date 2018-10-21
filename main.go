package main

import (
	"fmt"
	"io/ioutil"

	//"database/sql"
	"log"
	"net/http"

	"bytes"
	"encoding/json"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/satori/go.uuid"
)

type sUser struct {
	ID      int    `db:"id"`
	Login   string `db:"login"`
	Hash    string `db:"hash"`
	Session string `db:"session"`
}

type pAuth struct {
	Login string `json:"login"`
	Hash  string `json:"hash"`
}

var conn, err1 = sqlx.Connect("mysql", "root:mypass@tcp(localhost:3306)/dnd")

func main() {

	if err1 != nil {
		log.Printf(err1.Error())
		return
	}

	http.HandleFunc("/", handler)

	fmt.Println("Запускаемся. Слушаем порт 8080")

	http.ListenAndServe(":8080", nil)
}

func handler(iWrt http.ResponseWriter, iReq *http.Request) {
	fmt.Println("Request URL: " + iReq.URL.Path)
	var lGet = iReq.URL.Path[1:]
	fmt.Println("lGet " + lGet)
	switch lGet {
	case "":
		lGet = "index.html"
	case "/":
		lGet = "index.html"
	case "index.html":
		lGet = "index.html"
	case "style.css":
		iWrt.Header().Set("Content-Type", "text/css; charset=utf-8")
		lGet = "style.css"
	case "main.html":
		iWrt.WriteHeader(http.StatusBadRequest)
	case "reg":
		if iReq.Method == "POST" {

			var user sUser
			buf := new(bytes.Buffer)
			buf.ReadFrom(iReq.Body)
			fmt.Println(buf.String())
			var p pAuth

			err := json.Unmarshal(buf.Bytes(), &p)
			if err != nil {
				log.Printf(err.Error())
				return
			}

			fmt.Println(p.Login)
			err = conn.Get(&user, "select * from users where login=?", p.Login)
			if err != nil {
				log.Printf(err.Error())
				return
			}
			if user.Hash == p.Hash {

				session, err := uuid.NewV1()
				if err != nil {
					log.Printf(err.Error())
					return
				}
				user.Session = session.String()
				_, err = conn.Exec("update users set session =? where id=?", user.Session, user.ID)
				if err != nil {
					log.Printf(err.Error())
					return
				}
				fmt.Println(user.Session)
				lGet = "index.html"
			} else {
				iWrt.WriteHeader(http.StatusForbidden)
			}
		}
	}
	lData := readFile(lGet) // Считываем файл

	fmt.Fprintln(iWrt, lData)
}

func readFile(iFileName string) string {
	lData, err := ioutil.ReadFile(iFileName)
	var lOut string
	if err == nil {
		lOut = string(lData)
	} else {
		lOut = "404"
	}
	return lOut
}
