package main

import (
	"flag"
	"gortfolio/config"
	"gortfolio/database"
	"gortfolio/handlers"
	"gortfolio/pkg/chat"
	"gortfolio/pkg/shiritori"
	"gortfolio/trace"
	"gortfolio/utils"

	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/stretchr/objx"

	"github.com/stretchr/gomniauth"
	"github.com/stretchr/gomniauth/providers/google"
)

// templは1つのテンプレートを表します
type templateHandler struct {
	once     sync.Once
	filename string
	templ    *template.Template
}

// ServeHTTPはHTTPリクエストを処理します
func (t *templateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t.once.Do(func() {
		t.templ = template.Must(template.ParseFiles(filepath.Join("templates", t.filename)))
	})
	data := map[string]interface{}{}
	if authCookie, err := r.Cookie("auth"); err == nil {
		data["UserData"] = objx.MustFromBase64(authCookie.Value)
	}
	data["Msg"] = chat.GetMsgAll()
	_ = t.templ.Execute(w, data)
}

func main() {
	utils.LoggingSettings("chat.log")
	database.Migrate(chat.Message{})
	var addr = flag.String("addr", ":5002", "アプリケーションのアドレス")
	flag.Parse()
	// Gomniauthのセットアップ
	gomniauth.SetSecurityKey(config.Config.GomniauthKey)
	gomniauth.WithProviders(
		google.New(config.Config.GoogleClientID, config.Config.GoogleSecretValue, "http://localhost:5002/auth/callback/google"),
	)

	r := chat.NewRoom()
	r.Tracer = trace.New(os.Stdout)

	http.HandleFunc("/", handlers.Home)
	http.HandleFunc("/shiritori", shiritori.Handler)
	http.Handle("/chat", chat.MustAuth(&templateHandler{filename: "chat.html"}))
	http.Handle("/login", &templateHandler{filename: "login.html"})
	http.HandleFunc("/auth/", chat.LoginHandler)
	http.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:   "auth",
			Value:  "",
			Path:   "/",
			MaxAge: -1,
		})
		w.Header()["Location"] = []string{"/chat"}
		w.WriteHeader(http.StatusTemporaryRedirect)
	})
	http.Handle("/upload", &templateHandler{filename: "upload.html"})
	http.HandleFunc("/uploader", chat.UploaderHandler)
	http.Handle("/chat/avatars/",
		http.StripPrefix("/chat/avatars/", http.FileServer(http.Dir("chat/avatars"))))
	log.Println(http.Dir("chat/avatars"))
	http.Handle("/room", r)
	go r.Run()

	if err := http.ListenAndServe(*addr, nil); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}
