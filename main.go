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
	"github.com/gocraft/web"
	"github.com/jmoiron/sqlx"
	"github.com/satori/go.uuid"
)

const conf = "config.txt"

type Context struct {
	AdminToken string
}

type sUser struct {
	ID      int    `db:"id"`
	Login   string `db:"login"`
	Hash    string `db:"hash"`
	Session string `db:"session"`
	RoleId  int    `db:"roleId"`
}

type pAuth struct {
	Login string `json:"login"`
	Hash  string `json:"hash"`
}

type pReg struct {
	Login string `json:"login"`
	Hash  string `json:"hash"`
	Token string `json:"token"`
}

type sConfig struct {
	DBConnect  string `json:"DataBase"`
	AdminToken string `json:"Token"`
}

var Conn *sqlx.DB

func main() {
	file, err := ioutil.ReadFile(conf)
	if err != nil {
		log.Printf(err.Error())
		return
	}
	var config sConfig
	err = json.Unmarshal(file, &config)
	if err != nil {
		log.Printf(err.Error())
		return
	}
	lconn, err1 := sqlx.Connect("mysql", config.DBConnect)
	Conn = lconn
	if err1 != nil {
		log.Printf(err1.Error())
		return
	}

	router := web.New(Context{}).Middleware(web.LoggerMiddleware).Middleware(web.ShowErrorsMiddleware)
	router.Subrouter(Context{}, "/").Middleware((*Context).GetToken).Post("/reg", (*Context).Reg)
	router.Subrouter(Context{}, "/").Post("/auth", (*Context).Auth)

	fmt.Println("Запускаемся. Слушаем порт 8080")

	http.ListenAndServe(":8080", router)

}

func (c *Context) GetToken(iWrt web.ResponseWriter, iReq *web.Request, next web.NextMiddlewareFunc) {
	log.Println("Читаю токен")
	file, err := ioutil.ReadFile(conf)
	if err != nil {
		log.Printf(err.Error())
		return
	}
	var config sConfig
	err = json.Unmarshal(file, &config)
	if err != nil {
		log.Printf(err.Error())
		return
	}
	c.AdminToken = config.AdminToken
	next(iWrt, iReq)
	return
}

func (c *Context) Reg(iWrt web.ResponseWriter, iReq *web.Request) {
	log.Println("Регистрация")
	buf := new(bytes.Buffer)
	buf.ReadFrom(iReq.Body)
	fmt.Println(buf.String())
	var newUser pReg

	err := json.Unmarshal(buf.Bytes(), &newUser)
	if err != nil {
		log.Printf(err.Error())
		return
	}
	if c.AdminToken == newUser.Token {
		fmt.Println("Началась бд")
		var user sUser
		//user := new(sUser)
		err = Conn.Get(&user, "select * from users where login=?", newUser.Login)
		/*if err != nil {
			log.Println(err.Error())
			return
		}*/
		if err != nil && (err.Error() != "sql: no rows in result set") {
			log.Printf(err.Error())
			return
		}
		//fmt.Println("Нил?")

		if err.Error() == "sql: no rows in result set" {
			_, err = Conn.Exec("insert into users (login, hash) values (?,?)", newUser.Login, newUser.Hash)
			if err != nil {
				log.Printf(err.Error())
				return
			}
			resp, err := json.Marshal(newUser.Login)
			if err != nil {
				log.Printf(err.Error())
				return
			}
			_, err = iWrt.Write(resp)
			return

		}
		iWrt.WriteHeader(http.StatusBadRequest)
		return
	}
	log.Println("Токены не совпали")
	iWrt.WriteHeader(http.StatusBadRequest)
	return
}

func (c *Context) Auth(iWrt web.ResponseWriter, iReq *web.Request) {
	lData := new(bytes.Buffer)
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
	err = Conn.Get(&user, "select * from users where login=?", p.Login)
	if err != nil {
		log.Printf(err.Error())
		return
	}
	if user.Hash == p.Hash {

		session, err := uuid.NewV4()
		if err != nil {
			log.Printf(err.Error())
			return
		}
		user.Session = session.String()
		_, err = Conn.Exec("update users set session =? where id=?", user.Session, user.ID)
		if err != nil {
			log.Printf(err.Error())
			return
		}
		fmt.Println(user.Session)
		jsn, err := json.Marshal(user.Session)
		lData.Write(jsn)
		if err != nil {
			log.Printf(err.Error())
			return
		}
	} else {
		iWrt.WriteHeader(http.StatusForbidden)
		lData.Write([]byte("{}"))
	}

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
