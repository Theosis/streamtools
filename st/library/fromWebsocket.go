package library

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"github.com/nytlabs/streamtools/st/blocks" // blocks
	"github.com/nytlabs/streamtools/st/util"
	"net/http"
	"time"
)

// specify those channels we're going to use to communicate with streamtools
type FromWebsocket struct {
	blocks.Block
	queryrule chan chan interface{}
	inrule    chan interface{}
	inpoll    chan interface{}
	in        chan interface{}
	out       chan interface{}
	quit      chan interface{}
}

// we need to build a simple factory so that streamtools can make new blocks of this kind
func NewFromWebsocket() blocks.BlockInterface {
	return &FromWebsocket{}
}

// Setup is called once before running the block. We build up the channels and specify what kind of block this is.
func (b *FromWebsocket) Setup() {
	b.Kind = "FromWebsocket"
	b.inrule = b.InRoute("rule")
	b.queryrule = b.QueryRoute("rule")
	b.quit = b.Quit()
	b.out = b.Broadcast()
}

type recvHandler struct {
	toOut   chan interface{}
	toError chan error
}

func (self recvHandler) recv(ws *websocket.Conn, out chan interface{}) {
	for {
		_, p, err := ws.ReadMessage()
		if err != nil {
			self.toError <- err
			return
		}

		var outMsg interface{}
		err = json.Unmarshal(p, &outMsg)
		// if the json parsing fails, store data unparsed as "data"
		if err != nil {
			outMsg = map[string]interface{}{
				"data": p,
			}
		}
		self.toOut <- outMsg
	}
}

// Run is the block's main loop. Here we listen on the different channels we set up.
func (b *FromWebsocket) Run() {
	var ws *websocket.Conn
	var url string
	var handshakeDialer = &websocket.Dialer{
		Subprotocols: []string{"p1", "p2"},
	}
	listenWS := make(chan interface{})
	wsHeader := http.Header{"Origin": {"http://localhost/"}}

	toOut := make(chan interface{})
	toError := make(chan error)

	for {
		select {

		case msg := <-toOut:
			b.out <- msg

		case ruleI := <-b.inrule:
			var err error
			// set a parameter of the block
			url, err := util.ParseString(ruleI, "url")
			if err != nil {
				b.Error(err)
				continue
			}
			if ws != nil {
				ws.Close()
			}

			ws, _, err = handshakeDialer.Dial(url, wsHeader)
			if err != nil {
				b.Error("could not connect to url")
				break
			}
			ws.SetReadDeadline(time.Time{})
			h := recvHandler{toOut, toError}
			go h.recv(ws, listenWS)

		case err := <-toError:
			b.Error(err)

		case <-b.quit:
			// quit the block
			return
		case o := <-b.queryrule:
			o <- map[string]interface{}{
				"url": url,
			}
		case in := <-listenWS:
			b.out <- in
		}
	}
}
