// Copyright 2017 David Lazar. All rights reserved.
// Use of this source code is governed by the GNU AGPL
// license that can be found in the LICENSE file.

package pkg

import (
	"bytes"
	"encoding/json"
	"net/http"

	"golang.org/x/crypto/ed25519"
)

type statusArgs struct {
	Username         string
	Message          [32]byte
	ServerSigningKey ed25519.PublicKey `json:"-"`

	Signature []byte
}

func (a *statusArgs) msg() []byte {
	buf := new(bytes.Buffer)
	buf.WriteString("StatusArgs")
	buf.Write(a.ServerSigningKey)
	id := ValidUsernameToIdentity(a.Username)
	buf.Write(id[:])
	buf.Write(a.Message[:])
	return buf.Bytes()
}

type statusReply struct {
}

func (srv *Server) statusHandler(w http.ResponseWriter, req *http.Request) {
	args := new(statusArgs)
	err := json.NewDecoder(req.Body).Decode(args)
	if err != nil {
		httpError(w, errorf(ErrBadRequestJSON, "%s", err))
		return
	}
	args.ServerSigningKey = srv.publicKey

	reply, err := srv.checkStatus(args)
	if err != nil {
		httpError(w, err)
		return
	}

	bs, err := json.Marshal(reply)
	if err != nil {
		panic(err)
	}
	w.Write(bs)
}

func (srv *Server) checkStatus(args *statusArgs) (*statusReply, error) {
	_, err := UsernameToIdentity(args.Username)
	if err != nil {
		return nil, errorf(ErrInvalidUsername, "%s", err)
	}

	user, err := srv.getUser(nil, args.Username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errorf(ErrNotRegistered, "%q", args.Username)
	}
	if user.Status != statusVerified {
		return nil, errorf(ErrNotVerified, "%q", args.Username)
	}

	if !ed25519.Verify(user.Key, args.msg(), args.Signature) {
		return nil, errorf(ErrInvalidSignature, "")
	}

	return &statusReply{}, nil
}