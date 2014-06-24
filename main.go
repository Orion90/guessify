package main

import (
	"flag"
	"fmt"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
	"github.com/martini-contrib/sessions"
	"github.com/op/go-libspotify/spotify"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"
)

var (
	appKeyPath = flag.String("key", "spotify_appkey.key", "path to app.key")
	debug      = flag.Bool("debug", false, "debug output")
)

type Login struct {
	User string `form:"user"`
	Pass string `form:"pass"`
}

func main() {
	flag.Parse()
	m := martini.Classic()
	m.Map(setSession())
	store := sessions.NewCookieStore([]byte("secret123"))
	m.Use(sessions.Sessions("my_session", store))
	m.Use(martini.Static("assets"))
	m.Use(render.Renderer(render.Options{
		Directory:  "templates",                // Specify what path to load the templates from.
		Extensions: []string{".tmpl", ".html"}, // Specify extensions to load for templates.
		Charset:    "UTF-8",                    // Sets encoding for json and html content-types. Default is "UTF-8".
		IndentJSON: true,                       // Output human readable JSON
	}))
	m.Get("/", index)
	m.Get("/playlists", checkLogin, playlists)
	m.Get("/checkGuess/:track_id", checkLogin, checkGuess)
	m.Get("/playlist/:playlist_id", checkLogin, playlist)
	m.Get("/playlist/guess/:playlist_id", checkLogin, playlistGuess)
	m.Post("/", binding.Bind(Login{}), loginHandler)
	m.Get("/play/:playlist_id/:track_id", checkLogin, playTrack)
	logfile, _ := os.Create("log2.txt")
	defer logfile.Close()
	fmt.Fprintf(logfile, "%v", http.ListenAndServe(":3000", m))
}
func checkLogin(w http.ResponseWriter, r *http.Request, sp_session *spotify.Session) {

	if sp_session.ConnectionState() != spotify.ConnectionStateLoggedIn {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
}
func index(w http.ResponseWriter, r *http.Request, ren render.Render, sp_session *spotify.Session) {
	if sp_session.ConnectionState() != spotify.ConnectionStateLoggedIn {
		ren.HTML(200, "index", nil)
		return
	}
	http.Redirect(w, r, "/playlists", http.StatusFound)
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
		} else {
			http.Redirect(w, r, "/playlists", http.StatusOK)
		}
	}
}
func playTrack(w http.ResponseWriter, r *http.Request, params martini.Params, session *spotify.Session) {

	if session == nil {
		panic("Couldn't get session")
	}
	player := session.Player()
	player.Unload()
	playlists, err := session.Playlists()
	if err != nil {
		log.Fatal(err)
	}
	playlists.Wait()
	pid, _ := strconv.Atoi(params["playlist_id"])
	curr_playlist := playlists.Playlist(pid - 1)
	curr_playlist.Wait()
	tid, _ := strconv.Atoi(params["track_id"])
	curr_track := curr_playlist.Track(tid - 1).Track()

	curr_track.Wait()
	if err := player.Load(curr_track); err != nil {
		fmt.Println("%#v", err)
		log.Fatal(err)
	}
	pa := newPortAudio()
	session.SetAudioConsumer(pa)
	done := make(chan struct{})
	go pa.player(w, done)
	player.Play()
	defer player.Pause()
	defer player.Unload()
	for {
		select {
		case <-done:
			return
			break
		default:
		}
	}
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
type GuessContainer struct {
	Playlist string
	PlID     string
	Playing  byte
	Options  map[int]GuessOption
}
type GuessOption struct {
	TrackID string
	Name    string
	Artist  string
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

func playlistGuess(w http.ResponseWriter, r *http.Request, params martini.Params, session *spotify.Session, ren render.Render, usession sessions.Session) {
	playlists, err := session.Playlists()
	if err != nil {
		log.Fatal(err)
	}
	playlists.Wait()
	pid, _ := strconv.Atoi(params["playlist_id"])
	curr_playlist := playlists.Playlist(pid - 1)
	curr_playlist.Wait()
	ajax := false
	if r.FormValue("ajax") == "1" {
		ajax = true
	}

	rand.Seed(time.Now().UTC().UnixNano())
	trackToPlay := randInt(1, curr_playlist.Tracks())
	playingTrack := curr_playlist.Track(trackToPlay - 1)
	options := make(map[int]GuessOption, 4)
	options[trackToPlay-1] = GuessOption{
		playingTrack.Track().Link().String(),
		playingTrack.Track().Name(),
		playingTrack.Track().Artist(0).Name(),
	}
	for {
		if len(options) == 4 {
			break
		}
		rand.Seed(time.Now().UTC().UnixNano())
		newtrack := randInt(1, curr_playlist.Tracks())
		if _, ok := options[newtrack]; ok {
			continue
		}
		addTrack := curr_playlist.Track(newtrack - 1)
		options[newtrack-1] = GuessOption{
			addTrack.Track().Link().String(),
			addTrack.Track().Name(),
			addTrack.Track().Artist(0).Name(),
		}
	}
	usession.Set("playingTrack", playingTrack.Track().Link().String())
	if ajax {
		ren.HTML(200, "playlistGuessAjax", GuessContainer{Playlist: curr_playlist.Name(), PlID: params["playlist_id"], Playing: byte(trackToPlay), Options: options})
		return
	} else {
		ren.HTML(200, "playlistGuess", GuessContainer{Playlist: curr_playlist.Name(), PlID: params["playlist_id"], Playing: byte(trackToPlay), Options: options})
		return
	}
}
func checkGuess(w http.ResponseWriter, r *http.Request, params martini.Params, usession sessions.Session) {
	if params["track_id"] == usession.Get("playingTrack") {
		fmt.Fprintf(w, "OK")
	} else {
		fmt.Fprintf(w, usession.Get("playingTrack").(string))
	}
}
func randInt(min int, max int) int {
	return min + rand.Intn(max-min)
}
