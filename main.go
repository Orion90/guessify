package main

import (
	_ "fmt"
	"github.com/Orion90/spotifyweb"
	"github.com/go-martini/martini"
	"github.com/kr/pretty"
	"github.com/martini-contrib/render"
	"github.com/martini-contrib/sessions"
	"net/http"
)

type Login struct {
	LoginLink string
}

func setup(clientid, secret string) spotifyweb.SpotifyWeb {
	return spotifyweb.SpotifyWeb{
		Endpoint: "https://api.spotify.com/v1/",
		ClientID: clientid,
		Secret:   secret,
	}
}

func main() {
	m := martini.Classic()
	store := sessions.NewCookieStore([]byte("secret123"))
	m.Use(sessions.Sessions("spotify_session", store))
	m.Use(martini.Static("assets"))
	m.Map(setup("7e08b4f08cfd4f61b8903c94d245775f", "8c5b372be11e47b48da4ff27f0241cfb"))
	m.Use(render.Renderer(render.Options{
		Directory:  "templates",                // Specify what path to load the templates from.
		Extensions: []string{".tmpl", ".html"}, // Specify extensions to load for templates.
		Charset:    "UTF-8",                    // Sets encoding for json and html content-types. Default is "UTF-8".
		IndentJSON: true,                       // Output human readable JSON
	}))
	m.Get("/", checkLogin, index)
	m.Get("/login", login)
	m.Get("/auth", auth)
	http.ListenAndServe(":80", m)
}
func checkLogin(rw http.ResponseWriter, req *http.Request,
	s sessions.Session, c martini.Context, api spotifyweb.SpotifyWeb) {
	if s.Get("usertoken") == nil {
		http.Redirect(rw, req, "/login", http.StatusFound)
		return
	}
	api.Token = s.Get("usertoken").(string)
	me, err := api.Profile()
	if err != nil {
		pretty.Println(err)
	}
	c.Map(me)
}
func index(rend render.Render, api spotifyweb.SpotifyWeb, s sessions.Session, me spotifyweb.Me, c martini.Context) {
	pretty.Println(me)
	rend.HTML(200, "me", me)
}
func login(rend render.Render, api spotifyweb.SpotifyWeb) {
	rend.HTML(200, "index", Login{api.GetAuthUrl("http://localhost/auth")})
}
func auth(rw http.ResponseWriter, req *http.Request, s sessions.Session, api spotifyweb.SpotifyWeb, c martini.Context) {
	code := req.URL.Query().Get("code")
	token, _, err := api.GetToken(code, "http://localhost/auth")
	if err != nil {
		println(err)
	}
	s.Set("usertoken", token)
	http.Redirect(rw, req, "/", http.StatusFound)
}
