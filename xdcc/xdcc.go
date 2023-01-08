// Package xdcc plugin for webircgateway
package main

import (
	"bytes"
	"context"
	"log"
	"os"
	"os/exec"
	"regexp"

	"github.com/gorilla/mux"
	"github.com/gosimple/slug"
	"github.com/gotd/contrib/http_range"
	"golang.org/x/exp/maps"
	"gopkg.in/ini.v1"
   "net/url"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"unicode/utf8"

	"github.com/kiwiirc/webircgateway/pkg/irc"
	"github.com/kiwiirc/webircgateway/pkg/webircgateway"
	"golang.org/x/net/html/charset"
)

func remove[T comparable](l []T, item T) []T {
	for i, other := range l {
		if other == item {
			return append(l[:i], l[i+1:]...)
		}
	}
	return l
}

// Server muxer, dynamic map of handlers, and listen port.
type Server struct {
	Dispatcher *mux.Router
	fileNames  map[string]ParsedParts
	clientsMap map[string][]string
	Port       string
	server     http.Server
}
type XDCCConfig struct {
	Port                string
	DomainName          string
	LetsEncryptCacheDir string
	CertFile            string
	KeyFile             string
	server              Server
	TLS                 bool
}

var configs = XDCCConfig{
	Port:                "3000",
	DomainName:          func(n string, _ error) string { return n }(os.Hostname()),
	LetsEncryptCacheDir: "",
	CertFile:            "",
	KeyFile:             "",
	server: Server{Port: "3000", Dispatcher: mux.NewRouter(), fileNames: make(map[string]ParsedParts), clientsMap: make(map[string][]string), server: http.Server{
		Addr: "3000",
	}},
	TLS: false,
}

func int2ip(nn uint32) net.IP {
	ip := make(net.IP, 4)
	binary.BigEndian.PutUint32(ip, nn)
	return ip
}

type ParsedParts struct {
	ip             net.IP
	file           string
	port           int
	length         uint64
	receiverNick   string
	senderNick     string
	serverHostname string
	message        irc.Message
	upstreamSend   chan string
}

func parseSendParams(text string) *ParsedParts {
	re := regexp.MustCompile(`(?:[^\s"]+|"[^"]*")+`)
	replace := regexp.MustCompile(`^"(.+)"$`)

	parts := re.FindAllString(text, -1)

	ipInt, _ := strconv.ParseUint(parts[3], 10, 32)
	portInt, _ := strconv.ParseInt(parts[4], 10, 0)
	lengthInt, _ := strconv.ParseUint(parts[5], 10, 64)
	partsStruct := &ParsedParts{
		file:   replace.ReplaceAllString(parts[2], "$1"),
		ip:     int2ip(uint32(ipInt)),
		port:   int(portInt),
		length: lengthInt,
	}

	return partsStruct

}
func ensureUtf8(s string, fromEncoding string) string {
	if utf8.ValidString(s) {
		return s
	}

	encoding, encErr := charset.Lookup(fromEncoding)
	if encoding == nil {
		println("encErr:", encErr)
		return ""
	}

	d := encoding.NewDecoder()
	s2, _ := d.String(s)
	return s2
}

type WriteCounter struct {
	Total          uint64
	connection     *net.Conn
	expectedLength uint64
	writer         *io.PipeWriter
}

//	func reverseBytes(input []byte) []byte {
//	    if len(input) == 0 {
//	        return input
//	    }
//	    return append(reverseBytes(input[1:]), input[0])
//	}
func (wc *WriteCounter) Write(p []byte) (int, error) {
	n := len(p)
	wc.Total += uint64(n)
	buf := bytes.NewBuffer(make([]byte, 8))

	if wc.expectedLength > 0xffffffff {
		binary.Write((*wc.connection), binary.BigEndian, buf.Bytes())

	} else {
		binary.Write((*wc.connection), binary.BigEndian, buf.Bytes()[4:8])

	}
	if wc.expectedLength == wc.Total {
		(*wc.writer).Close()
	}
	return n, nil
}

func serveFile(parts ParsedParts, w http.ResponseWriter, r *http.Request) (work bool) {

	ipPort := fmt.Sprintf("%s:%d", parts.ip.String(), parts.port)
	//println(strings.Trim(m.GetParamU(1,""),"\x01"))
	//println(parts.ip.String())
	//	println(parts.port)
	if parts.ip == nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("404 - You tried"))
		return false
	}
	conn, err := net.Dial("tcp", ipPort)

	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(err.Error()))
		return false
	}

	pr, pw := io.Pipe()
	counter := &WriteCounter{
		connection:     &conn,
		Total:          0,
		expectedLength: parts.length,
		writer:         pw,
	}

	contentDisposition := fmt.Sprintf("attachment; filename=%s", parts.file)
	w.Header().Set("Content-Disposition", contentDisposition)
	w.Header().Set("Content-Type", "application/octet-stream")
	intLength := int(parts.length)
	if uint64(intLength) != parts.length {
		panic("overflows!")
	}
	w.Header().Set("Content-Length", strconv.Itoa(intLength) /*r.Header.Get("Content-Length")*/)

	go io.Copy(pw, io.TeeReader(conn, w))
	io.Copy(counter, pr)
	//stream the body to the client without fully loading it into memory
	// pbw := bufio.NewWriter(conn)
	// pbr := bufio.NewReader(conn)

	// req.Write(pbw)
	// pbw.Flush()

	// res, err := http.ReadResponse(pr, r)
	// if err != nil {
	// 	return nil, nil, err
	// }
	defer conn.Close()

	// go func() {
	//     // close the writer, so the reader knows there's no more data
	//     defer pw.Close()

	//     // write json data to the PipeReader through the PipeWriter
	//
	// }()
	return true

}
func DCCSend(hook *webircgateway.HookIrcLine) {

	//TODO DCC Send To Server
	if hook.Halt || hook.ToServer {
		return
	}
	client := hook.Client
	// Plugins may have modified the data
	data := hook.Line

	if data == "" {
		return
	}

	data = ensureUtf8(data, client.Encoding)
	if data == "" {
		return
	}
	m, parseErr := irc.ParseLine(data)
	if parseErr != nil {
		return
	}

	pLen := len(m.Params)

	if pLen > 0 && m.Command == "PRIVMSG" && strings.HasPrefix(strings.Trim(m.GetParamU(1, ""), "\x01"), "DCC SEND") { //can be moved to plugin goto hook.dispatch("irc.line")

		parts := parseSendParams(strings.Trim(m.GetParam(1, ""), "\x01"))
		parts.receiverNick = client.IrcState.Nick
		parts.senderNick = m.Prefix.Nick
		parts.serverHostname = client.UpstreamConfig.Hostname
		parts.message = *m
		parts.upstreamSend = client.UpstreamSend

		//TODO when file has no extension PARTS file
		lastIndex := strings.LastIndex(parts.file, ".")
		if lastIndex == -1 {
			lastIndex = len(parts.file)
		}

		parts.file = slug.Make(parts.receiverNick+strings.ReplaceAll(parts.serverHostname, ".", "_")+parts.senderNick+parts.file[0:lastIndex]) + parts.file[lastIndex:len(parts.file)] //long URLs may not work

		hook.Message.Command = "NOTICE"
		hook.Message.Params[1] = fmt.Sprintf("http://%s:3000/%s", configs.DomainName, parts.file)

		configs.server.AddFile(parts.file, *parts)

		client.SendClientSignal("data", hook.Message.ToLine())
	}

}

func DCCClose(hook *webircgateway.HookGatewayClosing) {

	configs.server.server.Shutdown(context.Background())

}
func ClientClose(hook *webircgateway.HookClientState) {
	if !hook.Connected {
		oldKeys := maps.Keys(configs.server.clientsMap)

		for i := range oldKeys {
			if strings.HasPrefix(oldKeys[i], hook.Client.IrcState.Nick+strings.ReplaceAll(hook.Client.UpstreamConfig.Hostname, ".", "_")) {
				delete(configs.server.clientsMap, oldKeys[i])
			}
		}

	}

}
func Start(gateway *webircgateway.Gateway, pluginsQuit *sync.WaitGroup) {
	gateway.Log(1, "XDCC plugin %s", webircgateway.Version)

	var configSrc interface{}

	if strings.HasPrefix(gateway.Config.ConfigFile, "$ ") {
		cmdRawOut, err := exec.Command("sh", "-c", gateway.Config.ConfigFile[2:]).Output()
		if err != nil {
			return
		}

		configSrc = cmdRawOut
	} else {
		configSrc = gateway.Config.ConfigFile
	}

	cfg, err := ini.LoadSources(ini.LoadOptions{AllowBooleanKeys: true, SpaceBeforeInlineComment: true}, configSrc)
	if err != nil {
		return
	}

	for _, section := range cfg.Sections() {
		if strings.Index(section.Name(), "XDCC") == 0 {

			configs.DomainName = section.Key("DomainName").MustString("")
			configs.TLS = section.Key("TLS").MustBool(false)
			configs.Port = section.Key("Port").MustString("3000")
			configs.LetsEncryptCacheDir = section.Key("LetsEncryptCacheDir").MustString("")
			configs.CertFile = section.Key("CertFile").MustString("")
			configs.KeyFile = section.Key("KeyFile").MustString("")

		}

	}

	if configs.TLS && configs.LetsEncryptCacheDir == "" {
		if configs.CertFile == "" || configs.KeyFile == "" {
			log.Print(3, "'cert' and 'key' options must be set for TLS servers")
			return
		}

		tlsCert := gateway.Config.ResolvePath(configs.CertFile)
		tlsKey := gateway.Config.ResolvePath(configs.KeyFile)

		log.Print(2, "XDCC: Listening with TLS on %s", configs.Port)
		keyPair, keyPairErr := tls.LoadX509KeyPair(tlsCert, tlsKey)
		if keyPairErr != nil {
			log.Print(3, "XDCC: Failed to listen with TLS, certificate error: %s", keyPairErr.Error())
			return
		}
		configs.server.server.Addr = configs.Port
		configs.server.server.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{keyPair},
		}

	} else if configs.TLS && configs.LetsEncryptCacheDir != "" {
		log.Print(2, "Listening with letsencrypt TLS on %s", configs.Port)
		leManager := gateway.Acme.Get(configs.LetsEncryptCacheDir)
		configs.server.server.Addr = configs.Port
		configs.server.server.TLSConfig = &tls.Config{
			GetCertificate: leManager.GetCertificate,
		}

	}

	webircgateway.HookRegister("irc.line", DCCSend)
	webircgateway.HookRegister("gateway.closing", DCCClose)
	webircgateway.HookRegister("client.state", ClientClose)

	// var port = flag.String("port", "3000", "Default: 3000; Set the port for the web-server to accept incoming requests")
	// flag.Parse()

	// server.Port = *port
	// log.Printf("Starting server on port: %s \n", server.Port)
	defer pluginsQuit.Done()
	configs.server.InitDispatch()
	log.Printf("XDCC: Initializing request routes...\n")

	go configs.server.Start() //Launch server; unblocks goroutine.

}

func (s *Server) Start() {
	log.Printf("XDCC: Listening on %s", s.Port)

	http.ListenAndServe(":"+s.Port, s.Dispatcher)
}

// InitDispatch routes.
func (s *Server) InitDispatch() {
	d := s.Dispatcher

	// Add handler to server's map.
	// d.HandleFunc("/register/{name}", func(w http.ResponseWriter, r *http.Request) { //map files to name
	//     //somewhere somehow you create the handler to be used; i'll just make an echohandler
	//     vars := mux.Vars(r)
	//     name := vars["name"]

	//     s.AddFile(w, r, name)
	// }).Methods("GET")

	// d.HandleFunc("/destroy/{name}", func(w http.ResponseWriter, r *http.Request) {
	//     vars := mux.Vars(r)
	//     name := vars["name"]
	//     s.Destroy(name)
	// }).Methods("GET")
	d.HandleFunc("/offline-first-example/dist/{name}", func(w http.ResponseWriter, r *http.Request) {
   u, err := url.Parse(r.Referer())
    if err != nil {
        panic(err)
    }
	stringArr := strings.Split(u.Path, "/")
	urlocator := fmt.Sprintf("http://%s:%s/%s", configs.DomainName,configs.Port, stringArr[0])
	temp := template.Must(template.ParseFiles("../offline-first-example/dist/work.bundle.js"))

	//set mime type to text/json
      err = temp.Execute(w, urlocator)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}).Methods("GET")
	d.HandleFunc("/{name}/video", func(w http.ResponseWriter, r *http.Request) {
		temp := template.Must(template.ParseGlob("../offline-first-example/dist/*"))
		
		
		err := temp.ExecuteTemplate(w, "indexPage", nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

	}).Methods("GET")
	d.HandleFunc("/{name}", func(w http.ResponseWriter, r *http.Request) {
		//Lookup handler in map and call it, proxying this writer and request
		vars := mux.Vars(r)
		name := vars["name"]

		// s.ProxyCall(w, r, name)

		parts := s.fileNames[name]
		ranges, err := http_range.ParseRange(r.Header.Get("Range"), int64(parts.length))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if len(ranges) > 0 {
			r := ranges[0]
			offset := r.Start
			// w.Header().Set("Content-Range", r.ContentRange(int64(parts.length)))
			// w.WriteHeader(http.StatusPartialContent)
			passLine := fmt.Sprintf(
				"DCC RESUME %s %d %d",
				parts.file,
				parts.port,
				offset,
			)
			parts.message.Params[1] = passLine
			// message, _ := irc.ParseLine(passLine)
			// message.

			parts.upstreamSend <- parts.message.ToLine()
		}

		//call serveFile here
		if serveFile(parts, w, r) { //removed go keyword this could mean servFile can only happen once

			//destroy route
			s.Destroy(parts)
		}

	}).Methods("GET")
}

func (s *Server) Destroy(parts ParsedParts) {
	delete(s.fileNames, parts.file)
	s.clientsMap[parts.receiverNick+strings.ReplaceAll(parts.serverHostname, ".", "_")+parts.senderNick] = remove(s.clientsMap[parts.receiverNick+strings.ReplaceAll(parts.serverHostname, ".", "_")+parts.senderNick], parts.file)
}

// func (s *Server) ProxyCall(w http.ResponseWriter, r *http.Request, fName string) {
//     if s.fileNames[fName] != nil {
//         s.fileNames[fName](w, r) //proxy the call
//     }
// }

func (s *Server) AddFile( /*w http.ResponseWriter, r *http.Request,*/ fName string, parts ParsedParts) { // add only 1 function instead
	// f := func(w http.ResponseWriter, r *http.Request) {
	//     w.Write([]byte("hello from" + fName))
	// }
	//store the parts and the hook
	s.fileNames[fName] = parts // Add the handler to our map

	configs.server.clientsMap[parts.receiverNick+strings.ReplaceAll(parts.serverHostname, ".", "_")+parts.senderNick] = append(configs.server.clientsMap[parts.receiverNick+strings.ReplaceAll(parts.serverHostname, ".", "_")+parts.senderNick], fName)

}
