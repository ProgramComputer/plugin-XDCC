# File sharing for [webircgateway](https://github.com/kiwiirc/webircgateway)
**A simple plugin to do xdcc for webircgateway to IRC networks for any web client**

### Overview
This plugin currently supports DCC SEND.


### Building and development

Build using
```console
go build -buildmode=plugin -o xdcc.so
```
in directory containing xdcc.go file.

Server runs on port 3000.

In config.conf,
under ```[plugins]``` put the path to xdcc.so file.
For example,
```console
[plugins]
./lorem/ipsum/plugin-XDCC.so
```
and under ``[XDCC]`` put a domain name or IP address without port when local.
For example,
```console
[plugins]
DomainName = lorem.ipsum.dolor.sit
```

Note- Currently SIGHUP on webircgateway will not reload this section. Webircgateway should be restarted.
## Commands
- [x] XDCC SEND
- [ ] XDCC RESUME
- [ ] XDCC ACCEPT
- [ ] XDCC REMOVE
- [x] XDCC CANCEL
- [ ] XDCC BATCH
- [ ] XDCC QUEUE
- [ ] XDCC INFO
- [ ] XDCC GET
- [x] XDCC STOP
- [x] XDCC HELP
- [x] XDCC SEARCH
## License
[ Licensed under the Apache License, Version 2.0](LICENSE).

