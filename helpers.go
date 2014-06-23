package main

import (
	_ "bufio"
	"encoding/binary"
	"flag"
	"fmt"
	"github.com/op/go-libspotify/spotify"
	"io"
	"net/http"
	_ "os"
	"os/exec"
	_ "sync"
)

type PlayInfo struct {
	Name     string
	User     string
	Playlist *spotify.Playlist
}

var (
	appKeyPath = flag.String("key", "/home/micke/Downloads/spotify_appkey.key", "path to app.key")
	debug      = flag.Bool("debug", false, "debug output")
)

type audio struct {
	format spotify.AudioFormat
	frames []byte
}

type audio2 struct {
	format spotify.AudioFormat
	frames []int16
}

type portAudio struct {
	buffer chan *audio
}

func newPortAudio() *portAudio {
	return &portAudio{
		buffer: make(chan *audio, 8),
	}
}

func (pa *portAudio) WriteAudio(format spotify.AudioFormat, frames []byte) int {
	audio := &audio{format, frames}

	if len(frames) == 0 {
		// println("no frames")
		return 0
	}

	select {
	case pa.buffer <- audio:
		// println("return", len(frames))
		return len(frames)
	default:
		// println("buffer full")
		return 0
	}
}

func (pa *portAudio) player(w http.ResponseWriter, done chan struct{}) {
	out := make([]int16, 2048*2)
	cmd := exec.Command("ffmpeg", "-i", "pipe:0", "-f", "mp3", "-")
	fw := flushWriter{w: w}
	if f, ok := w.(http.Flusher); ok {
		fw.f = f
	}
	cmd.Stdout = &fw
	reader, err := cmd.StdinPipe()
	defer reader.Close()
	if err != nil {
		chk(err)
	}

	// fmt.Println(cmd.Output())
	w.Header().Set("Content-Type", "audio/mpeg")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// reader.Header().Set("Content-Length", "5000000")

	// reader, _ := os.Create("tmpfile")
	// defer reader.Close()
	go func() {
		cmd.Run()
		done <- struct{}{}
	}()
	go func() {
		fmt.Fprintf(reader, "FORM")
		binary.Write(reader, binary.BigEndian, int32(5000000)) //total bytes
		fmt.Fprintf(reader, "AIFF")

		fmt.Fprintf(reader, "COMM")

		binary.Write(reader, binary.BigEndian, int32(18)) //size
		binary.Write(reader, binary.BigEndian, int16(1))  //channels
		binary.Write(reader, binary.BigEndian, int32(0))  //number of samples
		binary.Write(reader, binary.BigEndian, int16(32)) //bits per sample

		reader.Write([]byte{0x40, 0x0e, 0xac, 0x44, 0, 0, 0, 0, 0, 0}) //80-bit sample rate 44100

		fmt.Fprintf(reader, "SSND")

		binary.Write(reader, binary.BigEndian, int32(5000000)) //size
		binary.Write(reader, binary.BigEndian, int32(0))       //offset
		binary.Write(reader, binary.BigEndian, int32(0))       //block

		nSamples := 0
		for audio := range pa.buffer {
			if len(audio.frames) != 2048*2*2 {
				return
			}

			j := 0
			for i := 0; i < len(audio.frames); i += 2 {
				out[j] = int16(audio.frames[i]) | int16(audio.frames[i+1])<<8
				j++
			}
			binary.Write(reader, binary.BigEndian, out)
			nSamples += len(out)
			select {
			case <-done:
				return
				break
			default:
			}
		}
	}()
	for {
		select {
		case <-done:
			return
			break
		default:
		}
	}
}
func chk(err error) {
	if err != nil {
		panic(err)
	}
}

type flushWriter struct {
	f http.Flusher
	w io.Writer
}

func (fw *flushWriter) Write(p []byte) (n int, err error) {
	n, err = fw.w.Write(p)
	if fw.f != nil {
		fw.f.Flush()
	}
	return
}
