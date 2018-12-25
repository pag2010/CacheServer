package main

import (
	"fmt"
	"io/ioutil"
	"os"

	//"unicode/utf8"

	//"database/sql"
	"log"
	"net/http"

	//"bytes"
	"encoding/json"

	"strconv"
	//"strings"
	"path/filepath"
	"sync"
	"time"

	"regexp"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gocraft/web"
	"github.com/jmoiron/sqlx"
	"github.com/satori/go.uuid"
)

const conf = "config.txt"
const Path = "."

type httpError struct { //переопределить метод marshal
	Code int    `json:"code,omitempty"`
	Text string `json:"text,omitempty"`
}

type httpError2 struct { //переопределить метод marshal
	Code int    `json:"code,omitempty"`
	Text string `json:"text,omitempty"`
}

/*func (h *httpError) MarshalJSON() ([]byte, error) {
	if h.Code == 0 {
		return nil, nil
	}
	return json.Marshal(httpError2{Code: h.Code, Text: h.Text})
}*/

type Context struct {
	Err          *httpError `json:"error,omitempty"`
	Response     string     `json:"response,omitempty"`
	Data         string     `json:"data,omitempty"`
	File         *sFile     `json:"file,omitempty"`
	Files        []sFile    `json:"files,omitempty"`
	Login        string     `json:"-"`
	AnotherLogin string     `json:"-"`
}

type sUser struct {
	ID      int    `db:"id"` //nullint и nullstring в sql
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

type sFile struct {
	Id           string    `db:"id" json:"-"`
	Name         string    `db:"name" json:"name"`
	Body         string    `json:"body"`
	LastModified time.Time `json:"last_modified"`
	Size         int64     `json:"size"`
	Owner        string    `db:"owner" json:"owner"`
	Mime         string    `db:"mime" json:"mime,omitempty"`
	Grant        []string  `json:"grant,omitempty"`
	Public       bool      `db:"public" json:"public,omitempty"`
}

type sCache struct {
	m map[uint64]*sFile
	sync.RWMutex
}

var config sConfig
var Conn *sqlx.DB
var Cache sCache

func main() {
	err := LoadConfig()
	if err != nil {
		log.Printf(err.Error())
		return
	}

	Cache.m = make(map[uint64]*sFile)

	router := web.New(Context{}).Middleware(web.LoggerMiddleware).Middleware((*Context).ErrorHandler)
	router.Subrouter(Context{}, "/").Post("/reg", (*Context).Reg)
	router.Subrouter(Context{}, "/").Post("/auth", (*Context).Auth)
	router.Subrouter(Context{}, "/").Get("/rg", (*Context).CheckRegex)
	router.Subrouter(Context{}, "/").Get("/docs/:id", (*Context).GetDoc)
	router.Subrouter(Context{}, "/").Get("/docs", (*Context).GetDocs)
	//router.Subrouter(Context{}, "/").Middleware((*Context).ParseNewDoc).Post("/docs", (*Context).NewDoc)
	router.Subrouter(Context{}, "/").Middleware((*Context).ParseMultipartDoc).Post("/docs", (*Context).NewDoc)
	fmt.Println("Запускаемся. Слушаем порт 8080")
	http.Handle("/", router)
	err = http.ListenAndServe(":8080", nil)
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

	Conn, err = sqlx.Connect("mysql", config.DBConnect)
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
	rg := regexp.MustCompile(`^([a-z0-9_\.-]+)@([a-z0-9_\.-]+)\.([a-z\.]{2,6})$`)
	if b := rg.MatchString(newUser.Login); !b {
		c.SetError(400, "Логин не соответствует требованиям")
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
		var mode os.FileMode
		Filepath := filepath.Join(Path, "files", newUser.Login)
		_ = os.Mkdir(Filepath, mode)
		c.Response = newUser.Login
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
		c.Response = user.Session
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
	}

	lData, err := json.Marshal(c)
	if err != nil {
		iWrt.WriteHeader(500)
		fmt.Fprintln(iWrt, "")
	}
	fmt.Fprintln(iWrt, string(lData))
}

func (c *Context) SetError(code int, text string, args ...interface{}) {
	if text != "" {
		c.Err = new(httpError)
		c.Err.Code = code
		if len(args) == 0 {
			c.Err.Text = text
		} else {
			c.Err.Text = fmt.Sprintf(text, args...)
		}
	}
}

func (c *Context) GetDoc(iWrt web.ResponseWriter, iReq *web.Request) {
	c.Login = "Max"
	x, err := strconv.ParseUint(iReq.PathParams["id"], 10, 64)
	if err != nil {
		fmt.Println(err.Error())
		c.SetError(401, "Невозможно преобразовать id к числу")
		return
	}

	Cache.RLock()
	file, ok := Cache.m[x]

	Cache.RUnlock()
	if ok {
		c.File = new(sFile)
		c.File = file
		fmt.Println("Cache")
		return
	}

	fmt.Println("DB")
	files := []sFile{}
	err = Conn.Select(&files, "Select * from file where id=?", x)
	if err != nil {
		fmt.Println(err.Error())
		c.SetError(500, "Невозможно загрузить информацию о файле из БД")
		return
	}
	if len(files) != 1 {
		c.SetError(404, "Файл не найден")
		return
	}
	//gr := []string{}
	err = Conn.Select(&files[0].Grant, "Select user from usertofile where idfile=?", x)
	if err != nil {
		fmt.Println(err.Error())
		c.SetError(500, "Невозможно загрузить информацию о файле из БД")
		return
	}
	flag := false
	for _, g := range files[0].Grant {
		if g == c.Login {
			flag = true
			break
		}
	}
	if !flag {
		c.SetError(403, "Файл недоступен вам")
		return
	}
	c.File = new(sFile)
	c.File = &files[0]
	filepath := fmt.Sprint("./files/", c.File.Owner, "/", x)
	b, err := ioutil.ReadFile(filepath)
	if err != nil {
		fmt.Println(err.Error())
		c.SetError(500, "Невозможно считать файл")
		return
	}
	c.File.Body = string(b)
	FI, err := os.Stat(filepath)
	if err != nil {
		c.SetError(500, "Невозможно считать файл")
		return
	}
	c.File.LastModified = FI.ModTime()
	c.File.Size = FI.Size()
	Cache.Lock()
	Cache.m[x] = c.File
	Cache.Unlock()
}

func (c *Context) GetDocs(iWrt web.ResponseWriter, iReq *web.Request) {
	//c.AnotherLogin = "Max"
	c.Login = "Max"
	if c.AnotherLogin != "" {
		files := []sFile{}
		err := Conn.Select(&files, "select file.* from file  inner join usertofile on file.id=idfile where owner = ? and user = ?", c.AnotherLogin, c.Login)
		if err != nil {
			fmt.Println(err.Error())
			c.SetError(500, "Невозможно загрузить информацию о файле из БД")
			return
		}

		c.Files = files
		for i, f := range files {
			Filepath := filepath.Join(Path, "files", c.AnotherLogin, f.Id)
			b, err := ioutil.ReadFile(Filepath)
			if err != nil {
				fmt.Println(err.Error())
				c.SetError(404, "Файл не найден")
				continue
			}
			FI, err := os.Stat(Filepath)
			if err != nil {
				c.SetError(404, "Файл не найден")
				continue
			}
			c.Files[i].Body = string(b)
			c.Files[i].LastModified = FI.ModTime()
			c.Files[i].Size = FI.Size()
		}

	} else {
		filesDB := []sFile{}
		err := Conn.Select(&filesDB, "select * from file where owner = ?", c.Login)
		if err != nil {
			fmt.Println(err.Error())
			c.SetError(500, "Невозможно загрузить информацию о файле из БД")
			return
		}
		files, err := ioutil.ReadDir(filepath.Join(Path, "files", c.Login))
		if err != nil {
			c.SetError(500, "Невозможно считать файлы")
			return
		}
		c.Files = make([]sFile, len(files))
		for i, f := range files {
			Filepath := filepath.Join(Path, "files", c.Login, f.Name())
			b, err := ioutil.ReadFile(Filepath)
			if err != nil {
				c.SetError(404, "Файл не найден")
				continue
			}
			c.Files[i].Name = filesDB[i].Name
			c.Files[i].Body = string(b)
			c.Files[i].LastModified = f.ModTime()
			c.Files[i].Size = f.Size()
			c.Files[i].Owner = c.Login
			c.Files[i].Mime = filesDB[i].Mime
		}
	}

}

func (c *Context) NewDoc(iWrt web.ResponseWriter, iReq *web.Request) {
	c.Login = "Max"
	var mode os.FileMode
	Filepath := filepath.Join(Path, "files", c.Login)
	_ = os.Mkdir(Filepath, mode)
	var e error
	file := c.File
	c.File = nil
	files, err := ioutil.ReadDir(Filepath)

	Tx, err := Conn.Begin()
	defer Tx.Rollback()
	if err != nil {
		c.SetError(500, "Невозможно начать транзакцию с БД")
		return
	}
	r, err := Tx.Exec("Insert into file (name, owner, mime) values (?,?,?)", file.Name, c.Login, file.Mime)
	if err != nil {
		e = err
		c.SetError(500, "Ошибка транзакции при добавлении файла")
		Tx.Rollback()
		return
	}
	id, err := r.LastInsertId()
	if err != nil {
		e = err
		c.SetError(500, "Ошибка транзакции при добавлении. Информация не была добавлена")
		Tx.Rollback()
		return
	}
	for _, g := range file.Grant {
		_, err := Tx.Exec("Insert into usertofile values (?, ?)", g, id)
		if err != nil {
			e = err
			break
		}
	}

	if e != nil {
		fmt.Println(e.Error())
		c.SetError(500, "Ошибка в транзакции при добавлении пользователей")
		Tx.Rollback()
		return
	}
	for _, fi := range files {
		if fi.Name() == string(id) {
			c.SetError(401, "Такой файл уже есть")
			return
		}
	}
	var modef os.FileMode
	Filepath = filepath.Join(Filepath, fmt.Sprint(id))
	e = ioutil.WriteFile(Filepath, []byte(file.Body), modef)
	if e != nil {
		fmt.Println(e.Error())
		c.SetError(500, "Ошибка при создании файла")
		Tx.Rollback()
		return
	}
	fi, err := os.Stat(Filepath)
	if err != nil {
		c.SetError(500, "Ошибка при создании файла")
		Tx.Rollback()
		return
	}
	file.LastModified = fi.ModTime()
	file.Size = fi.Size()
	file.Name = fmt.Sprint(id)
	file.Owner = c.Login
	c.File = file
	err = Tx.Commit()
	if err != nil {
		c.SetError(500, "Невозможно завершить транзакцию")
		return
	}
}

func (c *Context) ParseNewDoc(iWrt web.ResponseWriter, iReq *web.Request, next web.NextMiddlewareFunc) {
	buf := json.NewDecoder(iReq.Body)
	defer iReq.Body.Close()
	var file sFile
	err := buf.Decode(&file)
	if err != nil {
		c.SetError(400, "Невозможно преобразовать тело запроса в json")
		return
	}
	c.File = &file
	fmt.Sprintf("%v", c.File)
	next(iWrt, iReq)
}

func (c *Context) ParseMultipartDoc(iWrt web.ResponseWriter, iReq *web.Request, next web.NextMiddlewareFunc) {
	err := iReq.ParseMultipartForm(0)
	if err != nil {
		c.SetError(401, "Невозможно считать файл из запроса")
		return
	}
	file, fileHeader, err := iReq.FormFile("doc")
	buf := make([]byte, fileHeader.Size)
	_, err = file.Read(buf) //ioutil.readall
	if err != nil {
		c.SetError(401, "Невозможно считать файл из запроса")
		return
	}
	c.File = new(sFile)
	c.File.Body = string(buf)
	c.File.Name = fileHeader.Filename
	c.File.Mime = "text/plain"
	c.File.Grant = []string{"Max", "Sasha"}
	c.File.Owner = "Sasha"
	next(iWrt, iReq)
}

func (c *Context) CheckRegex(iWrt web.ResponseWriter, iReq *web.Request) {
	rg := regexp.MustCompile(`^([a-z0-9_\.-]+)@([a-z0-9_\.-]+)\.([a-z\.]{2,6})$`)
	b := rg.MatchString(".ru")
	fmt.Println(b)
}
