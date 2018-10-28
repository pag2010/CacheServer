package main

import (
	"fmt"
	"io/ioutil"

	//"database/sql"
	"log"
	"net/http"

	//"bytes"
	"encoding/json"

	//"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gocraft/web"
	"github.com/jmoiron/sqlx"
	"github.com/satori/go.uuid"
)

const conf = "config.txt"

type httpError struct {
	Code int    `json:"code,omitempty"`
	Text string `json:"text,omitempty"`
}
type Context struct {
	Err      *httpError `json:"error,omitempty"`
	Response string     `json:"response,omitempty"`
	Data     string     `json:"data,omitempty"`
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

var config sConfig
var Conn *sqlx.DB

func main() {
	err := LoadConfig()
	if err != nil {
		log.Printf(err.Error())
		return
	}
	lconn, err := sqlx.Connect("mysql", config.DBConnect)
	Conn = lconn
	if err != nil {
		log.Printf(err.Error())
		return
	}

	router := web.New(Context{}).Middleware(web.LoggerMiddleware).Middleware((*Context).ErrorHandler)
	router.Subrouter(Context{}, "/").Post("/reg", (*Context).Reg)
	router.Subrouter(Context{}, "/").Post("/auth", (*Context).Auth)

	fmt.Println("Запускаемся. Слушаем порт 8080")

	err = http.ListenAndServe(":8080", router)
	if err != nil {
		log.Printf(err.Error())
		return
	}

}

func LoadConfig() error {
	file, err := ioutil.ReadFile(conf)
	if err != nil {
		log.Printf(err.Error())
		return err
	}
	err = json.Unmarshal(file, &config)
	if err != nil {
		log.Printf(err.Error())
		return err
	}
	return nil
}

func (c *Context) Reg(iWrt web.ResponseWriter, iReq *web.Request) {
	buf := json.NewDecoder(iReq.Body)
	defer iReq.Body.Close()

	var newUser pReg
	err := buf.Decode(&newUser)

	if err != nil {
		c.SetError(400, "Невозможно преобразовать тело запроса в json")
		log.Printf(err.Error())
		return
	}
	if config.AdminToken != newUser.Token {
		c.SetError(403, "Неверный хэш администратора")
		log.Println("Токены не совпали")
		return
	}

	fmt.Println("Началась бд")
	user := []sUser{}
	err = Conn.Select(&user, "select * from users where login=?", newUser.Login)

	if err != nil {
		c.SetError(500, "Ошибка базы данных")
		log.Printf(err.Error())
		return
	}

	if len(user) == 0 {
		_, err = Conn.Exec("insert into users (login, hash) values (?,?)", newUser.Login, newUser.Hash)
		if err != nil {
			log.Printf(err.Error())
			c.SetError(500, "Ошибка базы данных")
			return
		}
		c.Data = newUser.Login
		return

	} else {
		c.SetError(401, "Такой пользователь уже существует")
		return
	}
}

func (c *Context) Auth(iWrt web.ResponseWriter, iReq *web.Request) {
	var user sUser

	buf := json.NewDecoder(iReq.Body)
	defer iReq.Body.Close()

	var p pAuth

	err := buf.Decode(&p)
	if err != nil {
		c.SetError(400, "Невозможно преобразовать тело запроса в json")
		log.Printf(err.Error())
		return
	}

	fmt.Println(p.Login)
	err = Conn.Get(&user, "select * from users where login=?", p.Login)
	if err != nil {
		c.SetError(401, "Неверный логин или пароль")
		log.Printf(err.Error())
		return
	}
	if user.Hash == p.Hash {

		session, err := uuid.NewV4()
		if err != nil {
			c.SetError(500, "Не удалось создать сессию")
			log.Printf(err.Error())
			return
		}
		user.Session = session.String()
		_, err = Conn.Exec("update users set session =? where id=?", user.Session, user.ID)
		if err != nil {
			c.SetError(500, "Не удалось создать сессию")
			log.Printf(err.Error())
			return
		}
		c.Data = user.Session
		if err != nil {
			c.SetError(500, "Невозможно преобразовать ответ в json")
			log.Printf(err.Error())
			return
		}
	} else {
		c.SetError(403, "Неверный логин или пароль")
		return
	}
	return
}

func (c *Context) ErrorHandler(iWrt web.ResponseWriter, iReq *web.Request, next web.NextMiddlewareFunc) {
	next(iWrt, iReq)
	if c.Err != nil {
		iWrt.WriteHeader(c.Err.Code)
	} else {
		iWrt.WriteHeader(200)
	}
	lData, err := json.Marshal(c)
	if err != nil {
		iWrt.WriteHeader(500)
		fmt.Fprintln(iWrt, "")
	}
	fmt.Fprintln(iWrt, string(lData))
}

func (c *Context) SetError(code int, text string) {
	if text != "" {
		c.Err = new(httpError)
		c.Err.Code = code
		c.Err.Text = text
	}
}
func (c *Context) SendStatus(code int, text string, iWrt web.ResponseWriter) {
	if c.Err != nil {
		iWrt.WriteHeader(c.Err.Code)
	} else {
		iWrt.WriteHeader(200)
	}
	lData, err := json.Marshal(c)
	if err != nil {
		fmt.Fprintln(iWrt, "Все пропало")
		return
	}
	fmt.Println(string(lData))
	fmt.Fprintln(iWrt, string(lData))
}
