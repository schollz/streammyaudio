package server

import (
	"embed"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/dchest/captcha"
	"github.com/gabriel-vasile/mimetype"
	"github.com/h2non/filetype"
	log "github.com/schollz/logger"
	"github.com/schollz/streammyaudio/src/chat"
	"github.com/schollz/streammyaudio/src/filecreated"
)

//go:embed template
var templateFiles embed.FS

//go:embed static/*
var staticContent embed.FS

type Server struct {
	Port   int
	Folder string
}

type stream struct {
	b    []byte
	done bool
}

type view struct {
	Title     string
	Page      string
	FileNoExt string
	Items     []string
	Captcha   string
	Rand      string
	Archived  []ArchivedFile
	Message   string
	StripMP3  func(string) string
}

// Serve will start the server
func (s *Server) Run() (err error) {
	go chat.Run()

	tmpl := template.Must(template.ParseFS(templateFiles, "template/*"))

	channels := make(map[string]map[float64]chan stream)
	archived := make(map[string]*os.File)
	advertisements := make(map[string]bool)
	mutex := &sync.Mutex{}

	servePage := func(w http.ResponseWriter, r *http.Request, page string, msg string) (err error) {
		data := view{
			Page:    page,
			Message: msg,
			Rand:    fmt.Sprintf("%d", rand.Int31()),
			StripMP3: func(s string) string {
				return strings.TrimSuffix(s, ".mp3")
			},
		}

		switch page {
		case "live":
			adverts := []string{}
			mutex.Lock()
			for advert := range advertisements {
				adverts = append(adverts, strings.TrimPrefix(advert, "/"))
			}
			mutex.Unlock()
			data.Items = adverts
		case "archive":
			active := make(map[string]struct{})
			data.Archived = s.listArchived(active)
			data.Captcha = captcha.New()
		}
		log.Debugf("%s data: %+v", page, data)
		err = tmpl.ExecuteTemplate(w, page, data)
		if err != nil {
			panic(err)
		}
		return
	}
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		log.Debugf("opened %s %s", r.Method, r.URL.Path)
		defer func() {
			log.Debugf("finished %s\n", r.URL.Path)
		}()

		if r.URL.Path == "/" {
			servePage(w, r, "sya", "")
			return
		} else if r.URL.Path == "/favicon.ico" {
			w.WriteHeader(http.StatusOK)
			return
		} else if r.URL.Path == "/ws" {
			chat.Serve(w, r)
			return
		} else if r.Method == "POST" && strings.HasPrefix(r.URL.Path, "/archive") {
			filename := filepath.Clean(strings.TrimPrefix(r.FormValue("filename"), s.Folder))
			// This extra join implicitly does a clean and thereby prevents directory traversal
			filename = path.Join("/", filename)
			filename = path.Join(s.Folder, filename)
			action := r.FormValue("action")
			log.Debugf("%s %s", action, filename)
			if !captcha.VerifyString(r.FormValue("captchaId"), r.FormValue("captchaSolution")) {
				servePage(w, r, "archive", fmt.Sprintf("Incorrect captcha, could not %s '%s'", action, filename))
				return
			}
			msg := ""
			if action == "remove" {
				os.Remove(filename)
				msg = fmt.Sprintf("Removed '%s", filename)
			} else if action == "rename" {
				newname := strings.TrimSpace(r.FormValue("newname"))
				if !strings.HasSuffix(newname, ".mp3") {
					servePage(w, r, "archive", "Cannot rename suffix '.mp3'.")
					return
				}
				// This join with "/" prevents directory traversal with an implicit clean
				newname = path.Join("/", newname)
				newname = path.Join(s.Folder, newname)
				os.Rename(filename, newname)
				filename = strings.TrimPrefix(filename, "archived/")
				newname = strings.TrimPrefix(newname, "archived/")
				msg = fmt.Sprintf("Renamed '%s' to '%s'.", filename, newname)
			}
			servePage(w, r, "archive", msg)
			return
		} else if r.URL.Path == "/live" ||
			r.URL.Path == "/archive" ||
			r.URL.Path == "/download" {
			log.Debugf("serving page %s", r.URL.Path)
			servePage(w, r, r.URL.Path[1:], "")
			return

		} else if strings.HasPrefix(r.URL.Path, "/static/") {
			filename := filepath.Clean(strings.TrimPrefix(r.URL.Path, "/static/"))
			// This extra join implicitly does a clean and thereby prevents directory traversal
			filename = path.Join("/", filename)
			filename = path.Join("static", filename)
			log.Debugf("serving %s", filename)
			p, err := staticContent.ReadFile(filename)
			if err != nil {
				log.Error(err)
			}
			w.Write(p)
			return
		} else if !strings.HasSuffix(r.URL.Path, ".mp3") {
			data := view{
				Page:      "live",
				FileNoExt: r.URL.Path[1:],
				Rand:      fmt.Sprintf("%d", rand.Int31()),
			}
			err = tmpl.ExecuteTemplate(w, "chat", data)
			if err != nil {
				panic(err)
			}

			return
		}

		v, ok := r.URL.Query()["stream"]
		doStream := ok && v[0] == "true"

		v, ok = r.URL.Query()["archive"]
		doArchive := ok && v[0] == "true"

		if doArchive && r.Method == "POST" {
			if _, ok := archived[r.URL.Path]; !ok {
				folderName := path.Join(s.Folder, time.Now().Format("200601021504"))
				os.MkdirAll(folderName, os.ModePerm)
				archived[r.URL.Path], err = os.Create(path.Join(folderName, strings.TrimPrefix(r.URL.Path, "/")))
				if err != nil {
					log.Error(err)
				}
			}
			defer func() {
				mutex.Lock()
				if _, ok := archived[r.URL.Path]; ok {
					log.Debugf("closed archive for %s", r.URL.Path)
					archived[r.URL.Path].Close()
					delete(archived, r.URL.Path)
				}
				mutex.Unlock()
			}()
		}

		v, ok = r.URL.Query()["advertise"]
		log.Debugf("advertise: %+v", v)
		if ok && v[0] == "true" && doStream {
			mutex.Lock()
			advertisements[r.URL.Path] = true
			mutex.Unlock()
			defer func() {
				mutex.Lock()
				delete(advertisements, r.URL.Path)
				mutex.Unlock()
			}()
		}

		mutex.Lock()
		if _, ok := channels[r.URL.Path]; !ok {
			channels[r.URL.Path] = make(map[float64]chan stream)
		}
		mutex.Unlock()

		if r.Method == "GET" {
			id := rand.Float64()
			mutex.Lock()
			channels[r.URL.Path][id] = make(chan stream, 30)
			channel := channels[r.URL.Path][id]
			log.Debugf("added listener %f", id)
			mutex.Unlock()

			w.Header().Set("Connection", "keep-alive")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Cache-Control", "no-cache, no-store")

			mimetyped := false
			canceled := false
			for {
				select {
				case s := <-channel:
					if s.done {
						canceled = true
					} else {
						if !mimetyped {
							mimetyped = true
							mimetype := mimetype.Detect(s.b).String()
							if mimetype == "application/octet-stream" {
								ext := strings.TrimPrefix(filepath.Ext(r.URL.Path), ".")
								log.Debug("checking extension %s", ext)
								mimetype = filetype.GetType(ext).MIME.Value
							}
							w.Header().Set("Content-Type", mimetype)
							log.Debugf("serving as Content-Type: '%s'", mimetype)
						}
						w.Write(s.b)
						w.(http.Flusher).Flush()
					}
				case <-r.Context().Done():
					log.Debug("consumer canceled")
					canceled = true
				}
				if canceled {
					break
				}
			}

			mutex.Lock()
			delete(channels[r.URL.Path], id)
			log.Debugf("removed listener %f", id)
			mutex.Unlock()
			close(channel)
		} else if r.Method == "POST" {
			buffer := make([]byte, 2048)
			cancel := true
			isdone := false
			lifetime := 0
			for {
				if !doStream {
					select {
					case <-r.Context().Done():
						isdone = true
					default:
					}
					if isdone {
						log.Debug("is done")
						break
					}
					mutex.Lock()
					numListeners := len(channels[r.URL.Path])
					mutex.Unlock()
					if numListeners == 0 {
						time.Sleep(1 * time.Second)
						lifetime++
						if lifetime > 600 {
							isdone = true
						}
						continue
					}
				}
				n, err := r.Body.Read(buffer)
				if err != nil {
					log.Debugf("err: %s", err)
					if err == io.ErrUnexpectedEOF {
						cancel = false
					}
					break
				}
				if doArchive {
					mutex.Lock()
					archived[r.URL.Path].Write(buffer[:n])
					mutex.Unlock()
				}
				mutex.Lock()
				channels_current := channels[r.URL.Path]
				mutex.Unlock()
				for _, c := range channels_current {
					var b2 = make([]byte, n)
					copy(b2, buffer[:n])
					c <- stream{b: b2}
				}
			}
			if cancel {
				mutex.Lock()
				channels_current := channels[r.URL.Path]
				mutex.Unlock()
				for _, c := range channels_current {
					c <- stream{done: true}
				}
			}
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}

	log.Infof("running on port %d", s.Port)
	http.Handle("/archived/", http.FileServer(http.Dir(".")))
	http.Handle("/captcha/", captcha.Server(captcha.StdWidth, captcha.StdHeight))
	http.HandleFunc("/", handler)
	err = http.ListenAndServe(fmt.Sprintf(":%d", s.Port), nil)
	if err != nil {
		log.Error(err)
	}
	return
}

type ArchivedFile struct {
	Filename     string
	FullFilename string
	Created      time.Time
}

func (s *Server) listArchived(active map[string]struct{}) (afiles []ArchivedFile) {
	fnames := []string{}
	err := filepath.Walk(s.Folder,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				fnames = append(fnames, path)
			}
			return nil
		})
	if err != nil {
		return
	}
	for _, fname := range fnames {
		_, onlyfname := path.Split(fname)
		created := filecreated.FileCreated(fname)
		if _, ok := active[onlyfname]; !ok {
			afiles = append(afiles, ArchivedFile{
				Filename:     onlyfname,
				FullFilename: fname,
				Created:      created,
			})
		}
	}

	sort.Slice(afiles, func(i, j int) bool {
		return afiles[i].Created.After(afiles[j].Created)
	})

	return
}
