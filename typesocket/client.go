package typesocket

import (
	"encoding/json"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/websocket"
)

type clientConn struct {
	mu  sync.Mutex
	ws  *websocket.Conn
	mux Mux
}

type Conn interface {
	Send(msgID string, v interface{}) error

	Close() error
}

func Dial(addr string, mux Mux) (Conn, error) {
	dialer := &websocket.Dialer{
		HandshakeTimeout: 25 * time.Second,
	}
	ws, _, err := dialer.Dial(addr, nil)
	if err != nil {
		return nil, err
	}
	conn := &clientConn{
		ws:  ws,
		mux: mux,
	}
	go conn.readLoop()
	return conn, nil
}

func (c *clientConn) Close() error {
	return c.ws.Close()
}

func (c *clientConn) Send(msgID string, v interface{}) error {
	const writeWait = 10 * time.Second

	msg, err := json.Marshal(v)
	if err != nil {
		return err
	}
	e := &envelope{
		ID:      msgID,
		Message: msg,
	}

	c.mu.Lock()
	//c.ws.SetWriteDeadline(time.Now().Add(writeWait))
	if err := c.ws.WriteJSON(e); err != nil {
		log.WithFields(log.Fields{"call": "WriteJSON"}).Error(err)
		c.Close()
		return err
	}
	c.mu.Unlock()

	return nil
}

func (c *clientConn) readLoop() {
	for {
		var e envelope
		if err := c.ws.ReadJSON(&e); err != nil {
			log.WithFields(log.Fields{"call": "ReadJSON"}).Error(err)
			c.Close()
			break
		}
		go c.mux.openEnvelope(c, &e)
	}
}
