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
	//router := web.New(Context{}).Middleware(web.LoggerMiddleware).Middleware(web.ShowErrorsMiddleware).Middleware((*Context).GetToken).Post("/reg", (*Context).Reg)
	router := web.New(Context{}).Middleware(web.LoggerMiddleware).Middleware(web.ShowErrorsMiddleware)
	router.Subrouter(Context{}, "/").Middleware((*Context).GetToken).Post("/reg", (*Context).Reg)
	router.Subrouter(Context{}, "/").Post("/auth", (*Context).Auth)
	//router.Post("/auth", (*Context).Auth)
	/*http.HandleFunc("/auth1", handlerAuth)
	http.HandleFunc("/reg", handlerReg)
	http.HandleFunc("/", handler)*/
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

		if &user == nil {
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
			err = Conn.Get(&user, "select * from users where login=?", p.Login)
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
				_, err = Conn.Exec("update users set session =? where id=?", user.Session, user.ID)
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

func handlerAuth(iWrt http.ResponseWriter, iReq *http.Request) {
	lData := new(bytes.Buffer)
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

	} else {
		iWrt.WriteHeader(http.StatusMethodNotAllowed)
		lData.Write([]byte("{}"))
	}
	fmt.Fprintln(iWrt, lData)
}

func handlerReg(iWrt http.ResponseWriter, iReq *http.Request) {
	lData := new(bytes.Buffer)
	if iReq.Method == "POST" {
		buf := new(bytes.Buffer)
		buf.ReadFrom(iReq.Body)
		fmt.Println(buf.String())
		var newUser pReg

		err := json.Unmarshal(buf.Bytes(), &newUser)
		if err != nil {
			log.Printf(err.Error())
			return
		}
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
		fmt.Println(config.AdminToken)
		if config.AdminToken == newUser.Token {
			fmt.Println("Началась бд")
			var user sUser
			//user := new(sUser)
			err = Conn.Get(&user, "select * from users where login=?", newUser.Login)
			if (err != nil) && (err.Error() != "sql: no rows in result set") {
				log.Printf(err.Error())
				return
			}
			//fmt.Println("Нил?")
			if err != nil {
				fmt.Println(err.Error())
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
				lData.Write(resp)
			} else {
				iWrt.WriteHeader(http.StatusBadRequest)
				lData.Write([]byte("{}"))
			}
		}
	} else {
		iWrt.WriteHeader(http.StatusMethodNotAllowed)
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
