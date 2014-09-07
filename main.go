package main

import (
	"database/sql"
	"flag"
	"github.com/Orion90/spotifyweb"
	"github.com/coopernurse/gorp"
	"github.com/go-martini/martini"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/schema"
	"github.com/kr/pretty"
	"github.com/martini-contrib/render"
	"github.com/martini-contrib/sessions"
	"net/http"
	"strconv"
)

type Login struct {
	LoginLink string
}

var (
	host = flag.String("host", "localhost:8080", "Set the host.")
)

type Game struct {
	Id       int    `db:"game_id"`
	Name     string `db:"game_name" schema:"name"`
	Playlist string `db:"game_pl_id" schema:"playlist"`
	User     string `db:"game_user_id"`
	Step     string `db:"game_step"`
}

func SetupDB(c martini.Context) {
	var database_str = "root:6d7e6gk9@/guessify"
	db, _ := sql.Open("mysql", database_str)
	dbmap := &gorp.DbMap{Db: db, Dialect: gorp.MySQLDialect{"InnoDB", "UTF8"}}
	c.Map(dbmap)
}
func setup(clientid, secret string) spotifyweb.SpotifyWeb {
	return spotifyweb.SpotifyWeb{
		Endpoint: "https://api.spotify.com/v1/",
		ClientID: clientid,
		Secret:   secret,
	}
}

func main() {
	flag.Parse()
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
	m.Get("/", checkLogin, SetupDB, index)
	m.Get("/new", checkLogin, SetupDB, newGame)
	m.Post("/new", checkLogin, SetupDB, createNewGame)
	m.Get("/playlist/:id", checkLogin, playList)

	m.Get("/login", login)
	m.Get("/auth", auth)

	m.Get("/game/:id", checkLogin, SetupDB, playGame)

	http.ListenAndServe(*host, m)
}
func checkLogin(rw http.ResponseWriter,
	req *http.Request,
	s sessions.Session,
	c martini.Context,
	api spotifyweb.SpotifyWeb) {
	if s.Get("usertoken") == nil {
		http.Redirect(rw, req, "/login", http.StatusFound)
		return
	}
	api.Token = s.Get("usertoken").(string)
	me, err := api.Me()
	if err != nil {
		pretty.Println(err)
	}
	if me.Id == "" && s.Get("refreshtoken") != nil {
		token, _ := api.ReAuth(s.Get("refreshtoken").(string))
		s.Set("usertoken", token)
	} else if me.Id == "" {
		http.Redirect(rw, req, "/login", http.StatusFound)
		return
	}
	c.Map(me)
}
func playList(rend render.Render,
	api spotifyweb.SpotifyWeb,
	s sessions.Session,
	me spotifyweb.Me,
	c martini.Context,
	params martini.Params) {
	id, _ := strconv.Atoi(params["id"])
	pl := me.Playlists.Items[id]
	tracks, err := pl.GetFullTracks()
	if err != nil {
		pretty.Println(err)
	}
	data := PlaylistData{
		pl,
		tracks,
	}

	rend.HTML(200, "playlist", data)
}
func index(rend render.Render,
	api spotifyweb.SpotifyWeb,
	s sessions.Session,
	me spotifyweb.Me,
	c martini.Context) {

	rend.HTML(200, "me", me)
}
func login(rend render.Render,
	api spotifyweb.SpotifyWeb) {

	rend.HTML(200, "index", Login{api.GetAuthUrl("http://" + *host + "/auth")})
}
func auth(rw http.ResponseWriter,
	req *http.Request,
	s sessions.Session,
	api spotifyweb.SpotifyWeb,
	c martini.Context) {

	code := req.URL.Query().Get("code")
	token, refresh, err := api.GetToken(code, "http://"+*host+"/auth")
	if err != nil {
		println(err)
	}

	s.Set("usertoken", token)
	s.Set("refreshtoken", refresh)
	http.Redirect(rw, req, "/", http.StatusFound)
}
func newGame(rend render.Render,
	api spotifyweb.SpotifyWeb,
	s sessions.Session,
	me spotifyweb.Me,
	c martini.Context) {

	pretty.Println(me.Playlists)
	rend.HTML(200, "newgame", me)
}

func createNewGame(rw http.ResponseWriter,
	req *http.Request, rend render.Render,
	api spotifyweb.SpotifyWeb,
	s sessions.Session,
	me spotifyweb.Me,
	c martini.Context,
	db *gorp.DbMap) {

	defer db.Db.Close()
	req.ParseForm()
	data := new(Game)
	schema.NewDecoder().Decode(data, req.PostForm)
	data.User = me.Id
	data.Playlist
	db.AddTableWithName(Game{}, "t_game").SetKeys(true, "game_id")
	db.Insert(data)
	rend.HTML(200, "newgame", me)
}

func playGame(rw http.ResponseWriter,
	req *http.Request,
	rend render.Render,
	api spotifyweb.SpotifyWeb,
	s sessions.Session,
	me spotifyweb.Me,
	params martini.Params,
	db *gorp.DbMap) {

	defer db.Db.Close()
	var game Game
	id := params["id"]
	db.SelectOne(&game, "SELECT * FROM t_game WHERE game_id=?", id)
	pretty.Fprintf(rw, "%v", game)
}

func reauth(rw http.ResponseWriter,
	req *http.Request,
	s sessions.Session,
	api spotifyweb.SpotifyWeb) {

	me, _ := api.Me()
	if me.Id == "" {
		token, _ := api.ReAuth(s.Get("refreshtoken").(string))
		s.Set("usertoken", token)
	}
}

type PlaylistData struct {
	Playlist spotifyweb.PlaylistSimple
	Tracks   spotifyweb.TrackFullPagingObject
}
