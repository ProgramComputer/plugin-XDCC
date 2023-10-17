# File sharing for [webircgateway](https://github.com/kiwiirc/webircgateway)
**A simple plugin to do xdcc for webircgateway to IRC networks for any web client**

Add this to your web server to route XDCC commands.

### Overview
![demo](./demo.gif)

This plugin currently supports XDCC SEND. Append '/video' to url for video playback instead.


### Building and development

Build using
```console
go build -buildmode=plugin -o xdcc.so
```
in directory containing xdcc.go file.

File server runs on port 3000 by default.

In config.conf,
under ```[plugins]``` put the path to xdcc.so file.
For example,
```console
[plugins]
./lorem/ipsum/plugin-XDCC.so
```
and under ``[XDCC]`` set the following keys
- Port     3000
- DomainName (REQUIRED) is the domain Name of the server
- TLS bool
  - LetsEncryptCacheDir ""
  - CertFile ""
  - KeyFile ""

For example,
```console
[plugins]
DomainName = lorem.ipsum.dolor.sit
```

Note- Currently SIGHUP on webircgateway will not reload this section. Webircgateway should be restarted.
## Commands
- [ ] DCC SEND
- [ ] DCC Chat
- [ ] DCC Get
- [ ] DCC Reject
- [ ] DCC Ignore
- [ ] DCC nick
- [ ] DCC Passive
- [ ] DCC Trust
- [ ] DCC Maxcps 
- [x] XDCC SEND
- [ ] XDCC RESUME
- [ ] XDCC ACCEPT
- [ ] XDCC REMOVE
- [x] XDCC CANCEL
- [x] XDCC BATCH
- [ ] XDCC QUEUE
- [ ] XDCC INFO
- [ ] XDCC GET
- [x] XDCC STOP
- [x] XDCC HELP
- [x] XDCC SEARCH
## Contributions
Currently, only a few commands are supported. Any contributions to extend functionality are welcome.

## License
[ Licensed under the MIT License](LICENSE).

