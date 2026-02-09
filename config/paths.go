package config

// AssetPath is the path prefix for all assets. It indicates to the program to
// look for the given file in the assets collection of files
// which in development mode is wherever the file is on the disk, and in release mode, the central asset directory where
// all assets get copied.
var AssetPath = "/assets/"

var FrameworkSubPath = "serve"

// WebsocketMessengerPath is the url prefix that indicates this is a Websocket call to our messenger service.
//
// The default turns on Websockets and uses this to implement the Watcher and Messenger
// mechanisms. Override this in the goradd_project/config/goradd.go file to set to the value of your choice,
// or set to blank to turn off handling of websockets.
var WebsocketMessengerPath = "/ws/"

// ProxyPath is the url path to the application. By default, this is the root, but you can set it
// to any path. This is particularly useful to making the application appear as if it is running in a subdirectory
// of the root path. This is great for putting behind an Apache server, and using ProxyPass and ProxyPassReverse to direct
// traffic from a particular path to the application.
//
// Be sure to NOT strip this path from URLs at the server level.
// For example, in Apache, do this:
//
//	ProxyPass        /abc/ http://localhost:8080/abc/
//	ProxyPassReverse /abc/ http://localhost:8080/abc/
//
// and not
//
//	ProxyPass        /abc/ http://localhost:8080/
//	ProxyPassReverse /abc/ http://localhost:8080/
//
// because the standard Go MuxServer needs to know the full path in order to do its redirects.
// This also means you should NEVER use http.StripPath in front of a MUX handler.
// ProxyPath should start with a / character
var ProxyPath string
