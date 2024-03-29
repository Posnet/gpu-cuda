package transport

import (
	"bytes"
	"fmt"
	"github.com/metcalf/ctf3/level4/debuglog"
	"github.com/metcalf/raft"
	"io"
	"net/http"
	"net/url"
	"path"
)

// Parts from this transporter were heavily influenced by Peter Bougon's
// raft implementation: https://github.com/peterbourgon/raft

//------------------------------------------------------------------------------
//
// Typedefs
//
//------------------------------------------------------------------------------

// An HTTPTransporter is a default transport layer used to communicate between
// multiple servers.
type HTTPTransporter struct {
	DisableKeepAlives    bool
	prefix               string
	appendEntriesPath    string
	requestVotePath      string
	snapshotPath         string
	snapshotRecoveryPath string
	httpClient           http.Client
	Transport            *http.Transport
}

type HTTPMuxer interface {
	HandleFunc(string, func(http.ResponseWriter, *http.Request))
}

//------------------------------------------------------------------------------
//
// Constructor
//
//------------------------------------------------------------------------------

// Creates a new HTTP transporter with the given path prefix.
func NewHTTPTransporter(prefix string) *HTTPTransporter {
	t := &HTTPTransporter{
		DisableKeepAlives:    false,
		prefix:               prefix,
		appendEntriesPath:    joinPath(prefix, "/appendEntries"),
		requestVotePath:      joinPath(prefix, "/requestVote"),
		snapshotPath:         joinPath(prefix, "/snapshot"),
		snapshotRecoveryPath: joinPath(prefix, "/snapshotRecovery"),
		Transport: &http.Transport{
			Dial: UnixDialer,
		},
	}
	t.httpClient.Transport = t.Transport
	return t
}

//------------------------------------------------------------------------------
//
// Accessors
//
//------------------------------------------------------------------------------

// Retrieves the path prefix used by the transporter.
func (t *HTTPTransporter) Prefix() string {
	return t.prefix
}

// Retrieves the AppendEntries path.
func (t *HTTPTransporter) AppendEntriesPath() string {
	return t.appendEntriesPath
}

// Retrieves the RequestVote path.
func (t *HTTPTransporter) RequestVotePath() string {
	return t.requestVotePath
}

// Retrieves the Snapshot path.
func (t *HTTPTransporter) SnapshotPath() string {
	return t.snapshotPath
}

// Retrieves the SnapshotRecovery path.
func (t *HTTPTransporter) SnapshotRecoveryPath() string {
	return t.snapshotRecoveryPath
}

//------------------------------------------------------------------------------
//
// Methods
//
//------------------------------------------------------------------------------

//--------------------------------------
// Installation
//--------------------------------------

// Applies Raft routes to an HTTP router for a given server.
func (t *HTTPTransporter) Install(server raft.Server, mux HTTPMuxer) {
	mux.HandleFunc(t.AppendEntriesPath(), t.appendEntriesHandler(server))
	mux.HandleFunc(t.RequestVotePath(), t.requestVoteHandler(server))
	mux.HandleFunc(t.SnapshotPath(), t.snapshotHandler(server))
	mux.HandleFunc(t.SnapshotRecoveryPath(), t.snapshotRecoveryHandler(server))
}

//--------------------------------------
// Outgoing
//--------------------------------------

func debugAction(server raft.Server, peer *raft.Peer, method string, url string) {
	debuglog.Debugln(server.Name(), "->", peer.Name, "POST", url)
}

// Sends an AppendEntries RPC to a peer.
func (t *HTTPTransporter) SendAppendEntriesRequest(server raft.Server, peer *raft.Peer, req *raft.AppendEntriesRequest) *raft.AppendEntriesResponse {
	var b bytes.Buffer
	if _, err := req.Encode(&b); err != nil {
		debuglog.Debugln("transporter.ae.encoding.error:", err)
		return nil
	}

	url := joinPath(peer.ConnectionString, t.AppendEntriesPath())
	debugAction(server, peer, "POST", url)

	t.Transport.ResponseHeaderTimeout = server.ElectionTimeout()
	httpResp, err := t.httpClient.Post(url, "application/protobuf", &b)
	if httpResp == nil || err != nil {
		debuglog.Debugln("transporter.ae.response.error:", err)
		return nil
	}
	defer httpResp.Body.Close()

	resp := &raft.AppendEntriesResponse{}
	if _, err = resp.Decode(httpResp.Body); err != nil && err != io.EOF {
		debuglog.Debugln("transporter.ae.decoding.error:", err)
		return nil
	}

	return resp
}

// Sends a RequestVote RPC to a peer.
func (t *HTTPTransporter) SendVoteRequest(server raft.Server, peer *raft.Peer, req *raft.RequestVoteRequest) *raft.RequestVoteResponse {
	var b bytes.Buffer
	if _, err := req.Encode(&b); err != nil {
		debuglog.Debugln("transporter.rv.encoding.error:", err)
		return nil
	}

	url := fmt.Sprintf("%s%s", peer.ConnectionString, t.RequestVotePath())
	debugAction(server, peer, "POST", url)

	httpResp, err := t.httpClient.Post(url, "application/protobuf", &b)
	if httpResp == nil || err != nil {
		debuglog.Debugln("transporter.rv.response.error:", err)
		return nil
	}
	defer httpResp.Body.Close()

	resp := &raft.RequestVoteResponse{}
	if _, err = resp.Decode(httpResp.Body); err != nil && err != io.EOF {
		debuglog.Debugln("transporter.rv.decoding.error:", err)
		return nil
	}

	return resp
}

func joinPath(connectionString, thePath string) string {
	u, err := url.Parse(connectionString)
	if err != nil {
		panic(err)
	}
	u.Path = path.Join(u.Path, thePath)
	return u.String()
}

// Sends a SnapshotRequest RPC to a peer.
func (t *HTTPTransporter) SendSnapshotRequest(server raft.Server, peer *raft.Peer, req *raft.SnapshotRequest) *raft.SnapshotResponse {
	var b bytes.Buffer
	if _, err := req.Encode(&b); err != nil {
		debuglog.Debugln("transporter.rv.encoding.error:", err)
		return nil
	}

	url := joinPath(peer.ConnectionString, t.snapshotPath)
	debugAction(server, peer, "POST", url)

	httpResp, err := t.httpClient.Post(url, "application/protobuf", &b)
	if httpResp == nil || err != nil {
		debuglog.Debugln("transporter.rv.response.error:", err)
		return nil
	}
	defer httpResp.Body.Close()

	resp := &raft.SnapshotResponse{}
	if _, err = resp.Decode(httpResp.Body); err != nil && err != io.EOF {
		debuglog.Debugln("transporter.rv.decoding.error:", err)
		return nil
	}

	return resp
}

// Sends a SnapshotRequest RPC to a peer.
func (t *HTTPTransporter) SendSnapshotRecoveryRequest(server raft.Server, peer *raft.Peer, req *raft.SnapshotRecoveryRequest) *raft.SnapshotRecoveryResponse {
	var b bytes.Buffer
	if _, err := req.Encode(&b); err != nil {
		debuglog.Debugln("transporter.rv.encoding.error:", err)
		return nil
	}

	url := joinPath(peer.ConnectionString, t.snapshotRecoveryPath)
	debugAction(server, peer, "POST", url)

	httpResp, err := t.httpClient.Post(url, "application/protobuf", &b)
	if httpResp == nil || err != nil {
		debuglog.Debugln("transporter.rv.response.error:", err)
		return nil
	}
	defer httpResp.Body.Close()

	resp := &raft.SnapshotRecoveryResponse{}
	if _, err = resp.Decode(httpResp.Body); err != nil && err != io.EOF {
		debuglog.Debugln("transporter.rv.decoding.error:", err)
		return nil
	}

	return resp
}

//--------------------------------------
// Incoming
//--------------------------------------

// Handles incoming AppendEntries requests.
func (t *HTTPTransporter) appendEntriesHandler(server raft.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		debuglog.Debugln(server.Name(), "RECV /appendEntries")

		req := &raft.AppendEntriesRequest{}
		if _, err := req.Decode(r.Body); err != nil {
			http.Error(w, "", http.StatusBadRequest)
			return
		}

		resp := server.AppendEntries(req)
		if _, err := resp.Encode(w); err != nil {
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
	}
}

// Handles incoming RequestVote requests.
func (t *HTTPTransporter) requestVoteHandler(server raft.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		debuglog.Debugln(server.Name(), "RECV /requestVote")

		req := &raft.RequestVoteRequest{}
		if _, err := req.Decode(r.Body); err != nil {
			http.Error(w, "", http.StatusBadRequest)
			return
		}

		resp := server.RequestVote(req)
		if _, err := resp.Encode(w); err != nil {
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
	}
}

// Handles incoming Snapshot requests.
func (t *HTTPTransporter) snapshotHandler(server raft.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		debuglog.Debugln(server.Name(), "RECV /snapshot")

		req := &raft.SnapshotRequest{}
		if _, err := req.Decode(r.Body); err != nil {
			http.Error(w, "", http.StatusBadRequest)
			return
		}

		resp := server.RequestSnapshot(req)
		if _, err := resp.Encode(w); err != nil {
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
	}
}

// Handles incoming SnapshotRecovery requests.
func (t *HTTPTransporter) snapshotRecoveryHandler(server raft.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		debuglog.Debugln(server.Name(), "RECV /snapshotRecovery")

		req := &raft.SnapshotRecoveryRequest{}
		if _, err := req.Decode(r.Body); err != nil {
			http.Error(w, "", http.StatusBadRequest)
			return
		}

		resp := server.SnapshotRecoveryRequest(req)
		if _, err := resp.Encode(w); err != nil {
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
	}
}
