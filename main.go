package main

import (
	"fmt"
	"github.com/astaxie/beego/session"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
	"github.com/op/go-libspotify/spotify"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
)

var globalSessions *session.Manager

type Login struct {
	User string `form:"user"`
	Pass string `form:"pass"`
}

func main() {

	m := martini.Classic()
	globalSessions, _ = session.NewManager("memory", `{"cookieName":"gosessionid","gclifetime":3600}`)
	go globalSessions.GC()
	m.Map(setSession())
	m.Use(martini.Static("assets"))
	m.Use(render.Renderer(render.Options{
		Directory:  "templates",                // Specify what path to load the templates from.
		Extensions: []string{".tmpl", ".html"}, // Specify extensions to load for templates.
		Charset:    "UTF-8",                    // Sets encoding for json and html content-types. Default is "UTF-8".
		IndentJSON: true,                       // Output human readable JSON
	}))
	m.Get("/", index)
	m.Get("/playlists", checkLogin, playlists)
	m.Get("/playlist/:playlist_id", checkLogin, playlist)
	m.Post("/login", binding.Bind(Login{}), loginHandler)
	m.Get("/play/:playlist_id/:track_id", checkLogin, playTrack)
	m.Run()
}
func checkLogin(w http.ResponseWriter, r *http.Request, sp_session *spotify.Session) {

	if sp_session.ConnectionState() != spotify.ConnectionStateLoggedIn {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
}
func index(w http.ResponseWriter, r *http.Request, ren render.Render) {
	ren.HTML(200, "index", nil)
}
func setSession() *spotify.Session {
	appKey, err := ioutil.ReadFile(*appKeyPath)
	if err != nil {
		panic(err)
	}
	// pa := newPortAudio()
	sp_session, err := spotify.NewSession(&spotify.Config{
		ApplicationKey:   appKey,
		ApplicationName:  "prog",
		CacheLocation:    "tmp",
		SettingsLocation: "tmp",
		// AudioConsumer:    pa,
	})
	return sp_session
}
func loginHandler(w http.ResponseWriter, r *http.Request, params martini.Params, sp_session *spotify.Session, data Login) {

	sess := globalSessions.SessionStart(w, r)
	defer sess.SessionRelease(w)

	credentials := spotify.Credentials{
		Username: data.User,
		Password: data.Pass,
	}
	if err := sp_session.Login(credentials, false); err != nil {
		log.Fatal(err)
	}

	select {
	case err := <-sp_session.LoginUpdates():
		if err != nil {
			log.Fatal(err)
		}
	}
	fmt.Fprintf(w, "Session set!")
	http.Redirect(w, r, "/playlists", http.StatusFound)
}
func playTrack(w http.ResponseWriter, r *http.Request, params martini.Params, session *spotify.Session) {

	if session == nil {
		panic("Couldn't get session")
	}
	pa := newPortAudio()
	session.SetAudioConsumer(pa)
	done := make(chan struct{})
	go pa.player(w, done)
	playlists, err := session.Playlists()
	if err != nil {
		log.Fatal(err)
	}
	playlists.Wait()
	// playlist_count := playlists.Playlists()
	pid, _ := strconv.Atoi(params["playlist_id"])
	curr_playlist := playlists.Playlist(pid - 1)
	curr_playlist.Wait()
	tid, _ := strconv.Atoi(params["track_id"])
	curr_track := curr_playlist.Track(tid - 1).Track()

	curr_track.Wait()
	player := session.Player()
	if err := player.Load(curr_track); err != nil {
		fmt.Println("%#v", err)
		log.Fatal(err)
	}

	player.Play()
	defer player.Pause()
	for {
		select {
		case <-done:
			return
			break
		default:
		}
	}
	fmt.Println("Ended playing track")
}

type pl struct {
	Name   string
	Artist string
	Sort   int
}
type PlayContainer struct {
	Playlists []pl
}
type TrackContainer struct {
	Playlist string
	PlID     string
	Tracks   []pl
}

func playlists(w http.ResponseWriter, r *http.Request, params martini.Params, session *spotify.Session, ren render.Render) {
	playlists, err := session.Playlists()
	if err != nil {
		log.Fatal(err)
	}
	playlists.Wait()

	var playArr []pl
	for i := 0; i < playlists.Playlists(); i++ {
		playlist := playlists.Playlist(i)
		playlist.Wait()
		playArr = append(playArr, pl{playlist.Name(), "", i + 1})
	}

	ren.HTML(200, "playlists", PlayContainer{Playlists: playArr})
}

func playlist(w http.ResponseWriter, r *http.Request, params martini.Params, session *spotify.Session, ren render.Render) {
	playlists, err := session.Playlists()
	if err != nil {
		log.Fatal(err)
	}
	playlists.Wait()
	pid, _ := strconv.Atoi(params["playlist_id"])
	curr_playlist := playlists.Playlist(pid - 1)
	curr_playlist.Wait()

	var playArr []pl

	for i := 0; i < curr_playlist.Tracks(); i++ {
		track := curr_playlist.Track(i).Track()
		track.Wait()
		playArr = append(playArr, pl{track.Name(), track.Artist(0).Name(), i + 1})
	}

	ren.HTML(200, "playlist", TrackContainer{Tracks: playArr, Playlist: curr_playlist.Name(), PlID: params["playlist_id"]})
}
