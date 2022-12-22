// Package xdcc plugin for webircgateway
package main

import (
	"context"
	"log"
	"os"
	"os/exec"

	"github.com/gorilla/mux"
	"gopkg.in/ini.v1"

	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"

	"strings"
	"sync"
	"unicode/utf8"
    "crypto/tls"
	"github.com/kiwiirc/webircgateway/pkg/irc"
	"github.com/kiwiirc/webircgateway/pkg/webircgateway"
	"golang.org/x/net/html/charset"
)
type XDCCConfig struct {
	Port     string
	DomainName string
	LetsEncryptCacheDir string
	CertFile string
KeyFile string
server Server
TLS bool
}
var configs = XDCCConfig{
Port :"3000",
DomainName : func(n string, _ error) string { return n }(os.Hostname()),
LetsEncryptCacheDir : "",
CertFile: "",
KeyFile: "",
server: Server{Port: "3000", Dispatcher: mux.NewRouter(), fileNames: make(map[string]ParsedParts), server: http.Server{
	Addr: "3000",
	
}} ,
TLS: false,
}



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

		parts := parseSendParams(strings.Trim(m.GetParamU(1, ""), "\x01"))
		parts.file = client.IrcState.Nick + strings.ReplaceAll(client.UpstreamConfig.Hostname, ".", "_") + parts.file
		configs.server.AddFile(parts.file, *parts)
		log.Printf(parts.file)
		hook.Message.Command = "NOTICE"
		hook.Message.Params[1] = fmt.Sprintf("http://%s:3000/%s",configs.DomainName, parts.file)
		client.SendClientSignal("data", hook.Message.ToLine())
	}

}

func DCCClose() {

	configs.server.server.Shutdown(context.Background())

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
		configs.server.server.Addr = configs.Port;
		configs.server.server.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{keyPair},
		}
		

		
	} else if configs.TLS && configs.LetsEncryptCacheDir != "" {
		log.Print(2, "Listening with letsencrypt TLS on %s", configs.Port)
		leManager := gateway.Acme.Get(configs.LetsEncryptCacheDir)
		configs.server.server.Addr = configs.Port;
		configs.server.server.TLSConfig = &tls.Config{
			GetCertificate: leManager.GetCertificate,
		}
		
	}












	webircgateway.HookRegister("irc.line", DCCSend)
	webircgateway.HookRegister("gateway.closing", DCCClose)

	// var port = flag.String("port", "3000", "Default: 3000; Set the port for the web-server to accept incoming requests")
	// flag.Parse()

	// server.Port = *port
	// log.Printf("Starting server on port: %s \n", server.Port)
	defer pluginsQuit.Done()
	configs.server.InitDispatch()
	log.Printf("XDCC: Initializing request routes...\n")

	go configs.server.Start() //Launch server; unblocks goroutine.

}

// Server muxer, dynamic map of handlers, and listen port.
type Server struct {
	Dispatcher *mux.Router
	fileNames  map[string]ParsedParts
	Port       string
	server     http.Server
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
		s.Destroy(name) //TODO destroy when TIMEDOUT or  destroy when client use hook client.state

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
