package zenrpc

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type Printer interface {
	Printf(msg string, args ...any)
}

// printErr prints error if not nil.
func (s *Server) printErr(msg string, err error) {
	if err != nil {
		s.printf("%s: %v", msg, err)
	}
}

// httpError writes http header with status text.
func (s *Server) httpError(w http.ResponseWriter, code int) {
	http.Error(w, http.StatusText(code), code)
}

// ServeHTTP processes JSON-RPC 2.0 requests via HTTP.
// It handles CORS, SMD schema requests, and standard JSON-RPC calls.
// Implements http.Handler interface for the Server type.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// check for CORS GET & POST requests
	if s.options.AllowCORS {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	}

	// check for smd parameter and server settings and write schema if all conditions met,
	if _, ok := r.URL.Query()["smd"]; ok && s.options.ExposeSMD && r.Method == http.MethodGet {
		b, err := json.Marshal(s.SMD())
		s.printErr("json marshal", err)

		_, err = w.Write(b)
		s.printErr("response write", err)
		return
	}

	// check for CORS OPTIONS pre-requests for POST https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS
	if s.options.AllowCORS && r.Method == http.MethodOptions {
		w.Header().Set("Allow", "OPTIONS, GET, POST")
		w.Header().Set("Access-Control-Allow-Methods", "OPTIONS, GET, POST")
		w.Header().Set("Access-Control-Allow-Headers", "X-PINGOTHER, Content-Type")
		w.Header().Set("Access-Control-Max-Age", "86400")
		w.WriteHeader(http.StatusOK)
		return
	}

	// check for content-type and POST method.
	if !s.options.DisableTransportChecks {
		switch {
		case !strings.HasPrefix(r.Header.Get("Content-Type"), contentTypeJSON):
			s.httpError(w, http.StatusUnsupportedMediaType)
			return
		case r.Method == http.MethodGet:
			s.httpError(w, http.StatusMethodNotAllowed)
			return
		case r.Method != http.MethodPost:
			// skip rpc calls
			return
		}
	}

	// ok, method is POST and content-type is application/json, process body
	var data any
	b, err := io.ReadAll(r.Body)
	if err != nil {
		s.printf("read request body failed with err=%v", err)
		data = NewResponseError(nil, ParseError, "", nil)
	} else {
		data = s.process(NewRequestContext(r.Context(), r), b)
	}

	// if responses is empty -> all requests are notifications -> exit immediately
	if data == nil {
		return
	}

	// marshals data and write it to client.
	resp, err := json.Marshal(data)
	s.printErr("json marshal", err)
	if err != nil {
		s.httpError(w, http.StatusInternalServerError)
	}

	// write response
	w.Header().Set("Content-Type", contentTypeJSON)
	if _, err = w.Write(resp); err != nil {
		s.printErr("response write", err)
		s.httpError(w, http.StatusInternalServerError)
	}
}

// ServeWS processes JSON-RPC 2.0 requests via WebSocket using Gorilla WebSocket.
// It maintains a persistent connection and handles bidirectional JSON-RPC communication.
func (s *Server) ServeWS(w http.ResponseWriter, r *http.Request) {
	c, err := s.options.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.printf("upgrade connection failed with err=%v", err)
		return
	}
	defer func(c *websocket.Conn) {
		s.printErr("close websocket", c.Close())
	}(c)

	for {
		mt, message, err := c.ReadMessage()

		// normal closure
		if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
			break
		}
		// abnormal closure
		if err != nil {
			s.printf("read message failed with err=%v", err)
			break
		}

		data, err := s.Do(NewRequestContext(r.Context(), r), message)
		s.printErr("marshal json", err)
		if err != nil {
			e := c.WriteControl(websocket.CloseInternalServerErr, nil, time.Time{})
			s.printErr("write control", e)
			break
		}

		if err = c.WriteMessage(mt, data); err != nil {
			s.printf("write response failed with err=%v", err)
			e := c.WriteControl(websocket.CloseInternalServerErr, nil, time.Time{})
			s.printErr("write control", e)
			break
		}
	}
}

// SMDBoxHandler serves the SMDBox web application interface.
// This provides a web-based interface for exploring and testing the JSON-RPC API.
func SMDBoxHandler(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte(`
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>SMD Box</title>
    <link rel="stylesheet" href="https://bootswatch.com/3/paper/bootstrap.min.css">
	<link href="https://cdn.jsdelivr.net/gh/vmkteam/smdbox@latest/dist/app.css" rel="stylesheet"></head>
<body>
<div id="json-rpc-root"></div>
<script type="text/javascript" src="https://cdn.jsdelivr.net/gh/vmkteam/smdbox@latest/dist/app.js"></script></body>
</html>
	`))
}
