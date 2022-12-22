// Package xdcc plugin for webircgateway
package main

import (
	"log"

	"github.com/gorilla/mux"

	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"

	"strings"
	"sync"
	"unicode/utf8"

	"github.com/kiwiirc/webircgateway/pkg/irc"
	"github.com/kiwiirc/webircgateway/pkg/webircgateway"
	"golang.org/x/net/html/charset"
)

// Initialize Server
var server = &Server{Port: "3000", Dispatcher: mux.NewRouter(), fileNames: make(map[string]ParsedParts)} //moved from Start
func int2ip(nn uint32) net.IP {
	ip := make(net.IP, 4)
	binary.BigEndian.PutUint32(ip, nn)
	return ip
}

type ParsedParts struct {
	ip     net.IP
	file   string
	port   int
	length int64
}

func parseSendParams(text string) *ParsedParts {
	parts := strings.Split(text, " ")
	//re := regexp.MustCompile(`/(?:[^\s"]+|"[^"]*")+/g`)
	//fmt.Printf("%q\n", re.FindAllStringSubmatch(text, -1))
	//parts := text.match(/(?:[^\s"]+|"[^"]*")+/g);
	ipInt, _ := strconv.ParseUint(parts[3], 10, 32)
	portInt, _ := strconv.ParseInt(parts[4], 10, 0)
	lengthInt, _ := strconv.ParseInt(parts[5], 10, 64)
	partsStruct := &ParsedParts{
		file:   parts[2], /*.replace(/^"(.+)"$/, '$1')*/
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

func serveFile(parts ParsedParts, w http.ResponseWriter, r *http.Request) {

	ipPort := fmt.Sprintf("%s:%d", parts.ip.String(), parts.port)
	//println(strings.Trim(m.GetParamU(1,""),"\x01"))
	//println(parts.ip.String())
	//	println(parts.port)
	if parts.ip == nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("404 - You tried")) 
		return
	}
	conn, err := net.Dial("tcp", ipPort)
	if err != nil {
		return
	}

	pr, pw := io.Pipe()
	fmt.Println("I am in")

	contentDisposition := fmt.Sprintf("attachment; filename=%s", parts.file)
	w.Header().Set("Content-Disposition", contentDisposition)
	w.Header().Set("Content-Type", "application/octet-stream")
	intLength := int(parts.length)
	if int64(intLength) != parts.length {
		panic("overflows!")
	}
	w.Header().Set("Content-Length", strconv.Itoa(intLength) /*r.Header.Get("Content-Length")*/)
	go io.Copy(pw, conn)
	io.Copy(w, pr)
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
	//     if err := json.NewEncoder(pw).Encode(&PayLoad{Content: "Hello Pipe!"}); err != nil {
	//         log.Fatal(err)
	//     }
	// }()

	// JSON from the PipeWriter lands in the PipeReader
	// ...and we send it off...
	// if _, err := http.Post("http://example.com", "application/json", pr); err != nil {
	//     log.Fatal(err)
	// }
	// // 		url, _ := url.Parse("http://nginx-server/")
	// proxy := httputil.NewSingleHostReverseProxy(url)
	// proxy.FlushInterval = -1

	// //router.PathPrefix("/video").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	//        go proxy.ServeHTTP(w, r)
	//})
	//use the go keyword somewhere

	//	println(status)
}
func DCCSend(hook *webircgateway.HookIrcLine) {
	if hook.Halt {
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
		

		parts := parseSendParams(strings.Trim(m.GetParamU(1, ""), "\x01"))
        parts.file = client.IrcState.Nick + strings.ReplaceAll(client.UpstreamConfig.Hostname, ".", "_") + parts.file
		server.AddFile(parts.file, *parts)
		log.Printf(parts.file)
	}

}
func Start(gateway *webircgateway.Gateway, pluginsQuit *sync.WaitGroup) {
	gateway.Log(1, "XDCC plugin %s", webircgateway.Version)

	webircgateway.HookRegister("irc.line", DCCSend)

	// var port = flag.String("port", "3000", "Default: 3000; Set the port for the web-server to accept incoming requests")
	// flag.Parse()

	// server.Port = *port
	// log.Printf("Starting server on port: %s \n", server.Port)

	server.InitDispatch()
	log.Printf("Initializing request routes...\n")

	go server.Start() //Launch server; unblocks goroutine.

}

// Server muxer, dynamic map of handlers, and listen port.
type Server struct {
	Dispatcher *mux.Router
	fileNames  map[string]ParsedParts
	Port       string
}

func (s *Server) Start() {

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

	d.HandleFunc("/{name}", func(w http.ResponseWriter, r *http.Request) { 
		//Lookup handler in map and call it, proxying this writer and request
		vars := mux.Vars(r)
		name := vars["name"]

		// s.ProxyCall(w, r, name)

		parts := s.fileNames[name]

	

		//call serveFile here
		serveFile(parts, w, r) //removed go keyword this could mean servFile can only happen once

		//destroy route
		s.Destroy(name)

	}).Methods("GET")
}

func (s *Server) Destroy(fName string) {
	delete(s.fileNames, fName)

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
}
